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
//   - Match flag embedded in high bit of each transition value
//   - Match check in IsMatch: bitmap[stateIndex/64] & (1 << (stateIndex%64))
//   - Lookup: trans[sid + byteClass] — one addition, one load per byte
type DFA struct {
	// trans is the flat transition table with match flags in the high bit.
	// For non-match target states: value = premultiplied state ID.
	// For match target states: value = premultiplied state ID | matchFlag.
	// This allows the hot loop to check matches with a single AND operation,
	// while non-match states need no masking (high bit is 0 = clean ID).
	trans []uint32

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

	// maxPatternLen bounds searches once no longer match is possible.
	maxPatternLen int

	// matchKind specifies match semantics.
	matchKind MatchKind

	// startID is the premultiplied ID of the start state.
	startID uint32

	// startBytes contains all distinct bytes that appear at position 0 of any pattern.
	// Used as a prefilter: if none of these bytes exist in a haystack region,
	// no match can start there. Empty if too many start bytes (>3) or optimization
	// is not beneficial.
	startBytes []byte

	// patternBytes is a 256-bit bitmap of all bytes appearing in any pattern.
	// patternBytes[b/64] & (1 << (b%64)) != 0 means byte b appears in some pattern.
	// Used for prefilter: regions with no pattern bytes can be skipped.
	patternBytes [4]uint64
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

	// Store pattern lengths and compute prefilter data.
	d.patternLens = make([]int, len(patterns))
	startByteSet := [256]bool{}
	for i, p := range patterns {
		d.patternLens[i] = len(p)
		if len(p) > d.maxPatternLen {
			d.maxPatternLen = len(p)
		}
		if len(p) > 0 {
			startByteSet[p[0]] = true
		}
		for _, b := range p {
			d.patternBytes[b/64] |= 1 << (b % 64)
		}
	}

	// Collect start bytes for prefilter.
	for b := range 256 {
		if startByteSet[b] {
			d.startBytes = append(d.startBytes, byte(b))
		}
	}

	// Precompute which states are match states.
	isMatch := make([]bool, numStates)
	for si := range numStates {
		isMatch[si] = len(nfa.states[si].matches) > 0
	}

	// Build transition table with embedded match flags.
	tableSize := numStates * stride
	d.trans = make([]uint32, tableSize)

	for si := range numStates {
		rowOffset := si << stride2
		for class := range alphabetLen {
			next := resolveTransition(nfa, StateID(si), class)
			premultiplied := uint32(next) << stride2
			if isMatch[next] {
				premultiplied |= matchFlag
			}
			d.trans[rowOffset+class] = premultiplied
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
			d.matchIndex[si] = uint32(offset<<16) | uint32(count)
		} else {
			d.matchIndex[si] = 0xFFFFFFFF
			if d.matchOverflow == nil {
				d.matchOverflow = make(map[uint32][]PatternID)
			}
			d.matchOverflow[uint32(si)] = matches
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
		return d.matchOverflow[idx]
	}
	offset := int(packed >> 16)
	count := int(packed & 0xFFFF)
	return d.matchData[offset : offset+count]
}

// MemoryUsage returns the approximate heap memory used by this DFA in bytes.
func (d *DFA) MemoryUsage() int {
	return len(d.trans)*4 +
		len(d.matchIndex)*4 +
		len(d.matchData)*4 + len(d.patternLens)*8
}
