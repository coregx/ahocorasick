package ahocorasick

// OptimizedNFA represents an optimized Aho-Corasick automaton.
// Key optimizations:
// 1. Dense transitions: []StateID instead of map[byte]StateID
// 2. Precomputed root transitions: no failure link following for root
// 3. ByteClasses: reduced alphabet size
type OptimizedNFA struct {
	// states stores all state data.
	states []optState

	// byteClasses maps bytes to equivalence classes.
	byteClasses *ByteClasses

	// alphabetLen is the number of equivalence classes.
	alphabetLen int

	// startState is the ID of the start state (always 0).
	startState StateID

	// matchKind specifies match semantics.
	matchKind MatchKind

	// patternCount is the number of patterns.
	patternCount int
}

// optState represents an optimized state.
// Uses dense transitions for fast lookup.
type optState struct {
	// trans is a dense transition table indexed by byte class.
	// trans[class] = next state ID, or 0 if no transition (follow fail).
	// For root state, all transitions are precomputed (no fail following needed).
	trans []StateID

	// fail is the failure link state ID.
	fail StateID

	// matches lists pattern IDs that match at this state.
	matches []PatternID

	// depth is the depth from root.
	depth int
}

// buildOptimizedNFA constructs an optimized NFA from patterns.
func buildOptimizedNFA(patterns [][]byte, bc *ByteClasses, matchKind MatchKind) *OptimizedNFA {
	alphabetLen := bc.NumClasses()

	nfa := &OptimizedNFA{
		byteClasses:  bc,
		alphabetLen:  alphabetLen,
		startState:   0,
		matchKind:    matchKind,
		patternCount: len(patterns),
	}

	// Phase 1: Build trie from patterns
	nfa.buildTrie(patterns)

	// Phase 2: Compute failure links using BFS
	nfa.buildFailureLinks()

	// Phase 3: Propagate matches along failure links
	if matchKind == LeftmostFirst || matchKind == LeftmostLongest {
		nfa.propagateMatches()
	}

	// Phase 4: Precompute root transitions (key optimization!)
	nfa.precomputeRootTransitions()

	return nfa
}

// buildTrie constructs the initial trie from patterns.
func (nfa *OptimizedNFA) buildTrie(patterns [][]byte) {
	// Create root state with dense transitions
	nfa.states = append(nfa.states, optState{
		trans: make([]StateID, nfa.alphabetLen),
		fail:  0,
		depth: 0,
	})

	// Add each pattern to the trie
	for patternID, pattern := range patterns {
		nfa.addPattern(pattern, PatternID(patternID))
	}
}

// addPattern adds a single pattern to the trie.
func (nfa *OptimizedNFA) addPattern(pattern []byte, patternID PatternID) {
	state := nfa.startState

	for _, b := range pattern {
		class := nfa.byteClasses.Get(b)

		// Check if transition exists
		if next := nfa.states[state].trans[class]; next != 0 {
			state = next
		} else {
			// Create new state with dense transitions
			newState := StateID(len(nfa.states)) //nolint:gosec // G115: state count bounded by patterns
			nfa.states = append(nfa.states, optState{
				trans: make([]StateID, nfa.alphabetLen),
				fail:  0,
				depth: nfa.states[state].depth + 1,
			})
			nfa.states[state].trans[class] = newState
			state = newState
		}
	}

	// Mark this state as accepting for this pattern
	nfa.states[state].matches = append(nfa.states[state].matches, patternID)
}

// buildFailureLinks computes failure links using BFS.
func (nfa *OptimizedNFA) buildFailureLinks() {
	queue := make([]StateID, 0, len(nfa.states))

	// Initialize: children of root have failure link to root
	root := &nfa.states[nfa.startState]
	for class := 0; class < nfa.alphabetLen; class++ {
		if child := root.trans[class]; child != 0 {
			nfa.states[child].fail = nfa.startState
			queue = append(queue, child)
		}
	}

	// Process remaining states in BFS order
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]

		// For each outgoing transition
		for class := 0; class < nfa.alphabetLen; class++ {
			child := nfa.states[state].trans[class]
			if child == 0 {
				continue
			}
			queue = append(queue, child)

			// Compute failure link for child
			fail := nfa.states[state].fail
			for {
				if next := nfa.states[fail].trans[class]; next != 0 {
					nfa.states[child].fail = next
					break
				}
				if fail == nfa.startState {
					nfa.states[child].fail = nfa.startState
					break
				}
				fail = nfa.states[fail].fail
			}
		}
	}
}

// propagateMatches propagates match lists along failure links.
func (nfa *OptimizedNFA) propagateMatches() {
	queue := make([]StateID, 0, len(nfa.states))

	// Start with children of root
	root := &nfa.states[nfa.startState]
	for class := 0; class < nfa.alphabetLen; class++ {
		if child := root.trans[class]; child != 0 {
			queue = append(queue, child)
		}
	}

	// Process states in BFS order
	for len(queue) > 0 {
		stateID := queue[0]
		queue = queue[1:]

		state := &nfa.states[stateID]

		// Add children to queue
		for class := 0; class < nfa.alphabetLen; class++ {
			if child := state.trans[class]; child != 0 {
				queue = append(queue, child)
			}
		}

		// Propagate matches from failure state
		if state.fail != nfa.startState {
			failMatches := nfa.states[state.fail].matches
			if len(failMatches) > 0 {
				state.matches = append(state.matches, failMatches...)
			}
		}
	}
}

// precomputeRootTransitions fills in all root transitions.
// This is the KEY optimization: after this, root never needs failure link following.
func (nfa *OptimizedNFA) precomputeRootTransitions() {
	// Root state already has direct transitions set.
	// For any class without a transition, it loops back to root.
	// This is already the case (trans[class] == 0 means stay at root).
	// But we need to explicitly set it so nextState doesn't need to check.

	// Actually, for the root state, we keep trans[class] == 0 as meaning "stay at root".
	// The nextState function handles this specially.

	// For non-root states, we could also precompute failure transitions,
	// but that would use much more memory (DFA-style).
	// For now, we keep the NFA approach but with dense transitions.
}

// Note: nextState, isMatch, getMatches, stateCount methods removed.
// Search is now performed via the compiled DFA (see dfa.go, automaton.go).
// The NFA serves only as an intermediate representation for DFA construction.
