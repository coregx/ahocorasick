package ahocorasick

// matchFlag is set in the high bit of a transition value to indicate
// that the target state is a match state. This allows Find/FindAll to
// check for matches with a single bitwise AND, avoiding a separate lookup.
const matchFlag uint32 = 1 << 31

// matchMask clears the match flag to get the actual premultiplied state ID.
const matchMask uint32 = matchFlag - 1

// DFA represents a fully compiled deterministic finite automaton.
//
// Key properties:
//   - All failure transitions are pre-computed into the transition table
//   - Single flat []uint32 array for all transitions (cache-friendly)
//   - Premultiplied state IDs: sid = stateIndex << stride2
//   - Two transition tables: transClean (for IsMatch), transFlagged (for Find)
//   - Match check in IsMatch: bitmap[stateIndex/64] & (1 << (stateIndex%64))
//   - Lookup: trans[sid + byteClass] — one addition, one load per byte
type DFA struct {
	// trans is the clean transition table (no match flags).
	// Used by IsMatch for the tightest possible hot loop.
	trans []uint32

	// transFlagged has match flags in the high bit of each entry.
	// Used by Find/FindAll where we need to detect matches inline.
	transFlagged []uint32

	// matchBitmap is a compact bitmap: one bit per state.
	// matchBitmap[stateIndex/64] & (1 << (stateIndex%64)) != 0 means match.
	// Typically < 1KB, fits in L1 cache.
	matchBitmap []uint64

	// matchIndex maps state index to offset in matchData.
	// matchIndex[stateIdx] = (offset << 16) | count
	// If count == 0, state is not a match state.
	matchIndex []uint32

	// matchData stores all pattern IDs for match states, packed contiguously.
	matchData []PatternID

	// matchOverflow handles states where matchIndex encoding is insufficient.
	matchOverflow map[uint32][]PatternID

	// byteClasses maps bytes to equivalence classes.
	byteClasses *ByteClasses

	// alphabetLen is the number of equivalence classes.
	alphabetLen int

	// stride is the number of transitions per state (next power of 2 >= alphabetLen).
	stride int

	// stride2 is log2(stride). Used for bitshift: stateIndex = sid >> stride2.
	stride2 uint

	// stateCount is the total number of states.
	stateCount int

	// patternLens stores the length of each pattern (for computing match start).
	patternLens []int

	// matchKind specifies match semantics.
	matchKind MatchKind

	// startID is the premultiplied ID of the start state.
	startID uint32
}

// nextPow2 returns the smallest power of 2 >= n.
func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

// log2 returns floor(log2(n)) for n > 0.
func log2(n int) uint {
	var r uint
	for n >>= 1; n > 0; n >>= 1 {
		r++
	}
	return r
}

// buildDFA compiles a DFA from a noncontiguous NFA.
// The NFA must already have failure links and propagated matches.
func buildDFA(nfa *OptimizedNFA, patterns [][]byte, matchKind MatchKind) *DFA {
	numStates := len(nfa.states)
	alphabetLen := nfa.alphabetLen
	stride := nextPow2(alphabetLen)
	stride2 := log2(stride)

	d := &DFA{
		byteClasses: nfa.byteClasses,
		alphabetLen: alphabetLen,
		stride:      stride,
		stride2:     stride2,
		stateCount:  numStates,
		matchKind:   matchKind,
		startID:     uint32(nfa.startState) << stride2,
	}

	// Store pattern lengths.
	d.patternLens = make([]int, len(patterns))
	for i, p := range patterns {
		d.patternLens[i] = len(p)
	}

	// Build match bitmap: one bit per state.
	bitmapLen := (numStates + 63) / 64
	d.matchBitmap = make([]uint64, bitmapLen)
	for si := range numStates {
		if len(nfa.states[si].matches) > 0 {
			d.matchBitmap[si/64] |= 1 << uint(si%64)
		}
	}

	// Build both transition tables.
	tableSize := numStates * stride
	d.trans = make([]uint32, tableSize)
	d.transFlagged = make([]uint32, tableSize)

	for si := range numStates {
		rowOffset := si << stride2
		for class := range alphabetLen {
			next := resolveTransition(nfa, StateID(si), class) //nolint:gosec // bounded
			premultiplied := uint32(next) << stride2

			d.trans[rowOffset+class] = premultiplied

			// Add match flag for the flagged table.
			if len(nfa.states[next].matches) > 0 {
				d.transFlagged[rowOffset+class] = premultiplied | matchFlag
			} else {
				d.transFlagged[rowOffset+class] = premultiplied
			}
		}
	}

	// Pack match data contiguously.
	var totalMatches int
	for si := range numStates {
		totalMatches += len(nfa.states[si].matches)
	}

	d.matchData = make([]PatternID, 0, totalMatches)
	d.matchIndex = make([]uint32, numStates)

	for si := range numStates {
		matches := nfa.states[si].matches
		if len(matches) == 0 {
			continue
		}

		offset := len(d.matchData)
		count := len(matches)
		d.matchData = append(d.matchData, matches...)

		if offset <= 0xFFFF && count <= 0xFFFF {
			d.matchIndex[si] = uint32(offset<<16) | uint32(count) //nolint:gosec // bounded
		} else {
			d.matchIndex[si] = 0xFFFFFFFF
			if d.matchOverflow == nil {
				d.matchOverflow = make(map[uint32][]PatternID)
			}
			d.matchOverflow[uint32(si)] = matches //nolint:gosec // bounded
		}
	}

	return d
}

// resolveTransition follows failure links to find the effective transition
// for state s on byte class 'class'. This is done once at build time.
func resolveTransition(nfa *OptimizedNFA, s StateID, class int) StateID {
	for {
		if next := nfa.states[s].trans[class]; next != 0 {
			return next
		}
		if s == nfa.startState {
			return nfa.startState
		}
		s = nfa.states[s].fail
	}
}

// getMatches returns the pattern IDs that match at the given premultiplied state ID.
func (d *DFA) getMatches(sid uint32) []PatternID {
	idx := sid >> d.stride2
	packed := d.matchIndex[idx]
	if packed == 0 {
		return nil
	}
	if packed == 0xFFFFFFFF {
		return d.matchOverflow[uint32(idx)]
	}
	offset := int(packed >> 16)
	count := int(packed & 0xFFFF)
	return d.matchData[offset : offset+count]
}

// memoryUsage returns the approximate heap memory used by this DFA in bytes.
func (d *DFA) memoryUsage() int {
	return len(d.trans)*4 + len(d.transFlagged)*4 +
		len(d.matchBitmap)*8 + len(d.matchIndex)*4 +
		len(d.matchData)*4 + len(d.patternLens)*8
}
