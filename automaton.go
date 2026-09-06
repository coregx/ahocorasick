package ahocorasick

import "bytes"

// Automaton is the compiled Aho-Corasick multi-pattern matcher.
// Uses a fully compiled DFA with premultiplied state IDs for maximum throughput.
type Automaton struct {
	dfa       *DFA
	patterns  [][]byte
	matchKind MatchKind
}

// Find returns the first match in haystack starting at or after position start.
// Uses the flagged transition table for inline match detection.
// Prefilter: skips ahead using bytes.IndexByte when no start byte is nearby.
//
// Zero allocations — the Match is returned by value.
func (a *Automaton) Find(haystack []byte, start int) (Match, bool) {
	if start >= len(haystack) {
		return Match{}, false
	}

	d := a.dfa

	// Skip-ahead prefilter: jump directly to first start byte position.
	// Only engaged for haystacks >= 128 bytes to avoid overhead on short inputs.
	sb := d.startBytes
	remaining := len(haystack) - start
	if len(sb) > 0 && remaining >= 128 {
		skip := findEarliestStartByte(haystack[start:], sb)
		if skip < 0 {
			return Match{}, false
		}
		start += skip
	}

	trans := d.trans
	classes := &d.byteClasses.classes
	sid := d.startID
	patternLens := d.patternLens

	_ = trans[len(trans)-1]

	var bestMatch Match
	found := false

	for i := start; i < len(haystack); i++ {
		raw := trans[int(sid)+int(classes[haystack[i]])]

		// Common path (no match): raw has no flag, IS the clean state ID.
		// No masking needed — saves one AND per byte.
		if raw&matchFlag == 0 {
			sid = raw
			continue
		}

		// Rare path: match state reached. Clear the flag.
		sid = raw & matchMask
		matches := d.getMatches(sid)
		if len(matches) == 0 {
			continue
		}

		patternID := matches[0]
		matchEnd := i + 1
		matchStart := matchEnd - patternLens[patternID]

		m := Match{
			PatternID: int(patternID),
			Start:     matchStart,
			End:       matchEnd,
		}

		if a.matchKind == LeftmostFirst {
			return m, true
		}

		if !found || m.Len() > bestMatch.Len() {
			bestMatch = m
			found = true
		}
		if bestMatch.Len() == d.maxPatternLen {
			return bestMatch, true
		}
	}

	return bestMatch, found
}

// FindAt returns the first match starting exactly at position start.
//
// Zero allocations — the Match is returned by value.
func (a *Automaton) FindAt(haystack []byte, start int) (Match, bool) {
	if start >= len(haystack) {
		return Match{}, false
	}

	d := a.dfa
	trans := d.trans
	classes := &d.byteClasses.classes
	sid := d.startID
	startID := d.startID
	patternLens := d.patternLens

	_ = trans[len(trans)-1]

	var bestMatch Match
	found := false

	for i := start; i < len(haystack); i++ {
		prevSid := sid
		raw := trans[int(sid)+int(classes[haystack[i]])]

		if prevSid == startID && i > start {
			break
		}

		if raw&matchFlag == 0 {
			sid = raw
			continue
		}

		sid = raw & matchMask
		for _, patternID := range d.getMatches(sid) {
			patLen := patternLens[patternID]
			matchEnd := i + 1
			matchStart := matchEnd - patLen

			if matchStart != start {
				continue
			}

			m := Match{
				PatternID: int(patternID),
				Start:     matchStart,
				End:       matchEnd,
			}

			if a.matchKind == LeftmostFirst {
				return m, true
			}

			if !found || m.Len() > bestMatch.Len() {
				bestMatch = m
				found = true
			}
		}
	}

	return bestMatch, found
}

// IsMatch returns true if any pattern matches anywhere in the haystack.
// This is the most optimized search path — zero allocations, minimal branching.
//
// Uses a two-level prefilter strategy:
//  1. Skip-ahead: use SIMD bytes.IndexByte to jump directly to positions where
//     a match could start, skipping all non-pattern bytes in bulk.
//  2. DFA scan: run the automaton from the skip position to verify the match.
//
// If the automaton returns to start state during scanning, it re-engages
// the prefilter to skip ahead again. This is the same strategy as BurntSushi's
// Rust implementation.
func (a *Automaton) IsMatch(haystack []byte) bool {
	d := a.dfa

	// Skip-ahead prefilter: find the earliest position where any start byte occurs.
	// bytes.IndexByte is SIMD-optimized (~4ns per 64KB on amd64).
	// Since the DFA at start state transitions back to start for non-pattern bytes,
	// we can safely skip to the first start byte position.
	sb := d.startBytes
	if len(sb) > 0 {
		start := findEarliestStartByte(haystack, sb)
		if start < 0 {
			return false
		}
		haystack = haystack[start:]
	}

	trans := d.trans
	classes := &d.byteClasses.classes
	var sid uint32 // startID is always 0

	// BCE hint
	if len(trans) > 0 {
		_ = trans[len(trans)-1]
	}

	for i := 0; i < len(haystack); i++ {
		raw := trans[int(sid)+int(classes[haystack[i]])]
		if raw&matchFlag != 0 {
			return true
		}
		sid = raw

		// Re-engage prefilter when back at start state.
		// This skips large runs of non-pattern bytes between potential matches.
		if sid == 0 && len(sb) > 0 && i+1 < len(haystack) {
			skip := findEarliestStartByte(haystack[i+1:], sb)
			if skip < 0 {
				return false
			}
			i += skip // loop will i++ to land on the start byte
		}
	}

	return false
}

// findEarliestStartByte returns the earliest position in data where any of the
// start bytes occurs. Returns -1 if none found.
// Uses bytes.IndexByte which is SIMD-accelerated on amd64.
func findEarliestStartByte(data []byte, startBytes []byte) int {
	earliest := -1
	for _, b := range startBytes {
		if idx := bytes.IndexByte(data, b); idx >= 0 {
			if earliest < 0 || idx < earliest {
				earliest = idx
			}
		}
	}
	return earliest
}

// FindAll returns all non-overlapping matches in the haystack.
// If n >= 0, at most n matches are returned.
// Uses an inline DFA loop to avoid per-match heap allocations.
func (a *Automaton) FindAll(haystack []byte, n int) []Match {
	if len(haystack) == 0 {
		return nil
	}

	d := a.dfa
	trans := d.trans
	classes := &d.byteClasses.classes
	patternLens := d.patternLens
	var sid uint32 // startID = 0

	if len(trans) > 0 {
		_ = trans[len(trans)-1]
	}

	var matches []Match

	for i := 0; i < len(haystack); i++ {
		if n >= 0 && len(matches) >= n {
			break
		}

		raw := trans[int(sid)+int(classes[haystack[i]])]

		if raw&matchFlag == 0 {
			sid = raw
			continue
		}

		sid = raw & matchMask
		allMatches := d.getMatches(sid)
		if len(allMatches) == 0 {
			continue
		}

		// For LeftmostFirst, take the first pattern.
		patternID := allMatches[0]
		patLen := patternLens[patternID]
		matchEnd := i + 1
		matchStart := matchEnd - patLen

		matches = append(matches, Match{
			PatternID: int(patternID),
			Start:     matchStart,
			End:       matchEnd,
		})

		// Non-overlapping: skip past this match and reset to start state.
		if matchEnd > i+1 {
			i = matchEnd - 1 // loop will i++
		}
		sid = 0 // reset to start state
	}

	return matches
}

// FindAllOverlapping returns all overlapping matches in the haystack.
func (a *Automaton) FindAllOverlapping(haystack []byte) []Match {
	var matches []Match

	d := a.dfa
	trans := d.trans
	classes := &d.byteClasses.classes
	sid := d.startID
	patternLens := d.patternLens

	if len(trans) > 0 {
		_ = trans[len(trans)-1]
	}

	for i, b := range haystack {
		raw := trans[int(sid)+int(classes[b])]

		if raw&matchFlag == 0 {
			sid = raw
			continue
		}

		sid = raw & matchMask
		for _, patternID := range d.getMatches(sid) {
			matchEnd := i + 1
			matchStart := matchEnd - patternLens[patternID]

			matches = append(matches, Match{
				PatternID: int(patternID),
				Start:     matchStart,
				End:       matchEnd,
			})
		}
	}

	return matches
}

// Count returns the number of non-overlapping matches in the haystack.
func (a *Automaton) Count(haystack []byte) int {
	count := 0
	pos := 0

	for pos < len(haystack) {
		m, found := a.Find(haystack, pos)
		if !found {
			break
		}
		count++
		pos = m.End
		if pos <= m.Start {
			pos = m.Start + 1
		}
	}

	return count
}

// PatternCount returns the number of patterns in the automaton.
func (a *Automaton) PatternCount() int {
	return len(a.patterns)
}

// Pattern returns the pattern bytes at the given index.
func (a *Automaton) Pattern(id int) []byte {
	if id < 0 || id >= len(a.patterns) {
		return nil
	}
	return a.patterns[id]
}

// StateCount returns the number of states in the underlying automaton.
func (a *Automaton) StateCount() int {
	return a.dfa.stateCount
}

// MatchKind returns the match semantics used by this automaton.
func (a *Automaton) MatchKind() MatchKind {
	return a.matchKind
}
