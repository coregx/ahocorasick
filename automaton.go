package ahocorasick

// Automaton is the compiled Aho-Corasick multi-pattern matcher.
// Uses a fully compiled DFA with premultiplied state IDs for maximum throughput.
type Automaton struct {
	dfa       *DFA
	patterns  [][]byte
	matchKind MatchKind
}

// Find returns the first match in haystack starting at or after position start.
// Returns nil if no match is found.
// Uses the flagged transition table for inline match detection.
func (a *Automaton) Find(haystack []byte, start int) *Match {
	if start >= len(haystack) {
		return nil
	}

	d := a.dfa
	trans := d.transFlagged
	classes := &d.byteClasses.classes
	sid := d.startID
	patternLens := d.patternLens

	_ = trans[len(trans)-1]

	var bestMatch *Match

	for i := start; i < len(haystack); i++ {
		raw := trans[int(sid&matchMask)+int(classes[haystack[i]])]
		sid = raw

		if raw&matchFlag == 0 {
			continue
		}

		cleanSid := sid & matchMask
		matches := d.getMatches(cleanSid)
		if len(matches) == 0 {
			continue
		}

		patternID := matches[0]
		matchEnd := i + 1
		matchStart := matchEnd - patternLens[patternID]

		m := &Match{
			PatternID: int(patternID),
			Start:     matchStart,
			End:       matchEnd,
		}

		if a.matchKind == LeftmostFirst {
			return m
		}

		if bestMatch == nil || m.Len() > bestMatch.Len() {
			bestMatch = m
		}
	}

	return bestMatch
}

// FindAt returns the first match starting exactly at position start.
// Returns nil if no match starts at the given position.
func (a *Automaton) FindAt(haystack []byte, start int) *Match {
	if start >= len(haystack) {
		return nil
	}

	d := a.dfa
	trans := d.transFlagged
	classes := &d.byteClasses.classes
	sid := d.startID
	startID := d.startID
	patternLens := d.patternLens

	_ = trans[len(trans)-1]

	var bestMatch *Match

	for i := start; i < len(haystack); i++ {
		prevSid := sid & matchMask
		raw := trans[int(prevSid)+int(classes[haystack[i]])]
		sid = raw

		if prevSid == startID && i > start {
			break
		}

		if raw&matchFlag == 0 {
			continue
		}

		cleanSid := sid & matchMask
		for _, patternID := range d.getMatches(cleanSid) {
			patLen := patternLens[patternID]
			matchEnd := i + 1
			matchStart := matchEnd - patLen

			if matchStart != start {
				continue
			}

			m := &Match{
				PatternID: int(patternID),
				Start:     matchStart,
				End:       matchEnd,
			}

			if a.matchKind == LeftmostFirst {
				return m
			}

			if bestMatch == nil || m.Len() > bestMatch.Len() {
				bestMatch = m
			}
		}
	}

	return bestMatch
}

// IsMatch returns true if any pattern matches anywhere in the haystack.
// This is the most optimized search path — zero allocations, minimal branching.
//
// Uses the flagged transition table. Key insight: when raw has no match flag
// (the common case), raw IS the clean state ID — no masking needed.
// Only on match (return true) would masking be needed, but we don't use sid after.
//
// Hot loop per byte: 1 class lookup, 1 table lookup, 1 AND check.
func (a *Automaton) IsMatch(haystack []byte) bool {
	trans := a.dfa.transFlagged
	classes := &a.dfa.byteClasses.classes
	sid := a.dfa.startID // always 0

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
	}

	return false
}

// FindAll returns all non-overlapping matches in the haystack.
// If n >= 0, at most n matches are returned.
func (a *Automaton) FindAll(haystack []byte, n int) []Match {
	var matches []Match
	pos := 0

	for pos < len(haystack) && (n < 0 || len(matches) < n) {
		m := a.Find(haystack, pos)
		if m == nil {
			break
		}

		matches = append(matches, *m)

		pos = m.End
		if pos <= m.Start {
			pos = m.Start + 1
		}
	}

	return matches
}

// FindAllOverlapping returns all overlapping matches in the haystack.
func (a *Automaton) FindAllOverlapping(haystack []byte) []Match {
	var matches []Match

	d := a.dfa
	trans := d.transFlagged
	classes := &d.byteClasses.classes
	sid := d.startID
	patternLens := d.patternLens

	if len(trans) > 0 {
		_ = trans[len(trans)-1]
	}

	for i, b := range haystack {
		raw := trans[int(sid&matchMask)+int(classes[b])]
		sid = raw

		if raw&matchFlag == 0 {
			continue
		}

		cleanSid := sid & matchMask
		for _, patternID := range d.getMatches(cleanSid) {
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
		m := a.Find(haystack, pos)
		if m == nil {
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
