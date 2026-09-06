package ahocorasick

import (
	"bytes"
	"testing"
)

func TestBasicMatch(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"apple", "app", "maple"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		wantID   int
		wantPos  int
	}{
		{"I have an apple", 1, 10}, // "app" matches first (leftmost-first)
		{"maple syrup", 2, 0},
		{"application", 1, 0}, // "app"
		{"no match here", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			match, found := ac.Find([]byte(tt.haystack), 0)
			if tt.wantID < 0 {
				if found {
					t.Errorf("expected no match, got %+v", match)
				}
				return
			}
			if !found {
				t.Fatalf("expected match at %d, got none", tt.wantPos)
			}
			if match.PatternID != tt.wantID {
				t.Errorf("pattern ID = %d, want %d", match.PatternID, tt.wantID)
			}
			if match.Start != tt.wantPos {
				t.Errorf("start = %d, want %d", match.Start, tt.wantPos)
			}
		})
	}
}

func TestIsMatch(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		want     bool
	}{
		{"this is an error message", true},
		{"warning: something happened", true},
		{"fatal exception", true},
		{"all is well", false},
		{"err", false}, // partial match
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			got := ac.IsMatch([]byte(tt.haystack))
			if got != tt.want {
				t.Errorf("IsMatch(%q) = %v, want %v", tt.haystack, got, tt.want)
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"a", "ab", "abc"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("abc abc a")
	matches := ac.FindAll(haystack, -1)

	// With LeftmostFirst: "a" wins at each position
	// Positions: 0 (a), 4 (a), 8 (a)
	if len(matches) != 3 {
		t.Errorf("got %d matches, want 3", len(matches))
		for _, m := range matches {
			t.Logf("  match: pattern=%d pos=[%d:%d]", m.PatternID, m.Start, m.End)
		}
	}
}

func TestFindAllLimit(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"x"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("xxxxx")
	matches := ac.FindAll(haystack, 2)

	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}
}

func TestCount(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"the"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("the quick brown fox jumps over the lazy dog")
	count := ac.Count(haystack)

	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}

func TestEmptyPatternError(t *testing.T) {
	_, err := NewBuilder().
		AddStrings([]string{"valid", ""}).
		Build()
	if err == nil {
		t.Error("expected error for empty pattern")
	}
}

func TestNoPatternError(t *testing.T) {
	_, err := NewBuilder().Build()
	if err == nil {
		t.Error("expected error for no patterns")
	}
}

func TestOverlappingPatterns(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"he", "she", "his", "hers"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Classic Aho-Corasick test case
	haystack := []byte("ushers")
	matches := ac.FindAllOverlapping(haystack)

	// Should find: "she" at 1, "he" at 2, "hers" at 2
	if len(matches) < 2 {
		t.Errorf("got %d overlapping matches, want at least 2", len(matches))
		for _, m := range matches {
			t.Logf("  match: pattern=%d pos=[%d:%d] text=%q",
				m.PatternID, m.Start, m.End, haystack[m.Start:m.End])
		}
	}
}

func TestSinglePattern(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"needle"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		wantPos  int
	}{
		{"needle in haystack", 0},
		{"find the needle here", 9},
		{"no match", -1},
		{"needle", 0},
		{"needl", -1}, // partial
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			match, found := ac.Find([]byte(tt.haystack), 0)
			if tt.wantPos < 0 {
				if found {
					t.Errorf("expected no match, got %+v", match)
				}
				return
			}
			if !found {
				t.Fatalf("expected match at %d, got none", tt.wantPos)
			}
			if match.Start != tt.wantPos {
				t.Errorf("start = %d, want %d", match.Start, tt.wantPos)
			}
		})
	}
}

func TestByteClasses(t *testing.T) {
	patterns := [][]byte{
		[]byte("abc"),
		[]byte("xyz"),
	}
	bc := NewByteClasses(patterns)

	// Only 'a', 'b', 'c', 'x', 'y', 'z' should have non-zero classes
	usedBytes := map[byte]bool{'a': true, 'b': true, 'c': true, 'x': true, 'y': true, 'z': true}

	for i := 0; i < 256; i++ {
		class := bc.Get(byte(i))
		if usedBytes[byte(i)] {
			if class == 0 {
				t.Errorf("byte %q should have non-zero class", byte(i))
			}
		} else {
			if class != 0 {
				t.Errorf("byte %d should have class 0, got %d", i, class)
			}
		}
	}

	// Should have 7 classes: 0 (unused) + 6 (a,b,c,x,y,z)
	if bc.NumClasses() != 7 {
		t.Errorf("NumClasses() = %d, want 7", bc.NumClasses())
	}
}

func TestLiteralAlternation(t *testing.T) {
	// This is the target use case for Issue #48
	ac, err := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal", "critical"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("2024-01-01 [error] something failed\n2024-01-01 [warning] something suspicious\n2024-01-01 [fatal] crash!")

	matches := ac.FindAll(haystack, -1)
	if len(matches) != 3 {
		t.Errorf("got %d matches, want 3", len(matches))
	}

	// Verify patterns
	expected := []string{"error", "warning", "fatal"}
	for i, m := range matches {
		if i >= len(expected) {
			break
		}
		got := string(haystack[m.Start:m.End])
		if got != expected[i] {
			t.Errorf("match %d: got %q, want %q", i, got, expected[i])
		}
	}
}

// Edge case tests

func TestSingleBytePatterns(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"a", "b", "c"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		wantID   int
		wantPos  int
	}{
		{"a", 0, 0},
		{"b", 1, 0},
		{"c", 2, 0},
		{"abc", 0, 0},
		{"xbx", 1, 1},
		{"xxc", 2, 2},
		{"xyz", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			match, found := ac.Find([]byte(tt.haystack), 0)
			if tt.wantID < 0 {
				if found {
					t.Errorf("expected no match, got %+v", match)
				}
				return
			}
			if !found {
				t.Fatalf("expected match, got none")
			}
			if match.PatternID != tt.wantID {
				t.Errorf("PatternID = %d, want %d", match.PatternID, tt.wantID)
			}
			if match.Start != tt.wantPos {
				t.Errorf("Start = %d, want %d", match.Start, tt.wantPos)
			}
		})
	}
}

func TestLongPatterns(t *testing.T) {
	// Create a 100-byte pattern
	longPattern := bytes.Repeat([]byte("abcdefghij"), 10)

	ac, err := NewBuilder().
		AddPattern(longPattern).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Haystack contains the pattern
	haystack := append([]byte("prefix"), longPattern...)
	haystack = append(haystack, []byte("suffix")...)

	match, found := ac.Find(haystack, 0)
	if !found {
		t.Fatal("expected match for long pattern")
	}
	if match.Start != 6 {
		t.Errorf("Start = %d, want 6", match.Start)
	}
	if match.End != 106 {
		t.Errorf("End = %d, want 106", match.End)
	}
}

func TestPrefixPatterns(t *testing.T) {
	// Patterns that are prefixes of each other
	ac, err := NewBuilder().
		AddStrings([]string{"a", "ab", "abc", "abcd"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// LeftmostFirst should return "a"
	match, found := ac.Find([]byte("abcd"), 0)
	if !found {
		t.Fatal("expected match")
	}
	if match.PatternID != 0 {
		t.Errorf("PatternID = %d, want 0 (pattern 'a')", match.PatternID)
	}
}

func TestSuffixPatterns(t *testing.T) {
	// Patterns that are suffixes of each other
	ac, err := NewBuilder().
		AddStrings([]string{"d", "cd", "bcd", "abcd"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("abcd")
	matches := ac.FindAllOverlapping(haystack)

	// Should find all four patterns
	if len(matches) != 4 {
		t.Errorf("got %d matches, want 4", len(matches))
		for _, m := range matches {
			t.Logf("  match: pattern=%d text=%q", m.PatternID, haystack[m.Start:m.End])
		}
	}
}

func TestFailureLinkCorrectness(t *testing.T) {
	// Classic failure link test: "abcab" searching for "ab"
	ac, err := NewBuilder().
		AddStrings([]string{"ab"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// "abcab" has "ab" at positions 0 and 3
	haystack := []byte("abcab")
	matches := ac.FindAll(haystack, -1)

	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}
	if len(matches) >= 2 {
		if matches[0].Start != 0 {
			t.Errorf("first match at %d, want 0", matches[0].Start)
		}
		if matches[1].Start != 3 {
			t.Errorf("second match at %d, want 3", matches[1].Start)
		}
	}
}

func TestUnicodePatterns(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"日本", "東京", "京都"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		want     bool
	}{
		{"私は日本に住んでいます", true},
		{"東京は大きい", true},
		{"京都は美しい", true},
		{"hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			got := ac.IsMatch([]byte(tt.haystack))
			if got != tt.want {
				t.Errorf("IsMatch(%q) = %v, want %v", tt.haystack, got, tt.want)
			}
		})
	}
}

func TestBinaryPatterns(t *testing.T) {
	// Patterns with null bytes and binary data
	ac, err := NewBuilder().
		AddPatterns([][]byte{
			{0x00, 0x01, 0x02},
			{0xFF, 0xFE, 0xFD},
			{0x00, 0x00, 0x00},
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack []byte
		want     bool
	}{
		{[]byte{0x00, 0x01, 0x02, 0x03}, true},
		{[]byte{0xAA, 0xFF, 0xFE, 0xFD}, true},
		{[]byte{0x00, 0x00, 0x00}, true},
		{[]byte{0x01, 0x02, 0x03}, false},
	}

	for i, tt := range tests {
		got := ac.IsMatch(tt.haystack)
		if got != tt.want {
			t.Errorf("test %d: IsMatch = %v, want %v", i, got, tt.want)
		}
	}
}

func TestFindAt(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"abc", "ab", "a"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		haystack string
		start    int
		wantID   int
	}{
		{"xabc", 0, -1}, // no match starting at 0
		{"xabc", 1, 2},  // "a" at position 1
		{"abc", 0, 2},   // "a" at position 0
	}

	for _, tt := range tests {
		t.Run(tt.haystack, func(t *testing.T) {
			match, found := ac.FindAt([]byte(tt.haystack), tt.start)
			if tt.wantID < 0 {
				if found {
					t.Errorf("expected no match, got %+v", match)
				}
				return
			}
			if !found {
				t.Fatal("expected match, got none")
			}
			if match.PatternID != tt.wantID {
				t.Errorf("PatternID = %d, want %d", match.PatternID, tt.wantID)
			}
		})
	}
}

func TestLeftmostLongest(t *testing.T) {
	ac, err := NewBuilder().
		SetMatchKind(LeftmostLongest).
		AddStrings([]string{"a", "ab", "abc"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	match, found := ac.Find([]byte("abc"), 0)
	if !found {
		t.Fatal("expected match")
	}
	// LeftmostLongest should return "abc" (pattern 2), not "a" (pattern 0)
	if match.PatternID != 2 {
		t.Errorf("PatternID = %d, want 2 (pattern 'abc')", match.PatternID)
	}
}

func TestAutomatonAccessors(t *testing.T) {
	patterns := []string{"foo", "bar", "baz"}
	ac, err := NewBuilder().
		AddStrings(patterns).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// PatternCount
	if got := ac.PatternCount(); got != 3 {
		t.Errorf("PatternCount() = %d, want 3", got)
	}

	// Pattern
	if got := ac.Pattern(0); string(got) != "foo" {
		t.Errorf("Pattern(0) = %q, want %q", got, "foo")
	}
	if got := ac.Pattern(1); string(got) != "bar" {
		t.Errorf("Pattern(1) = %q, want %q", got, "bar")
	}
	if got := ac.Pattern(2); string(got) != "baz" {
		t.Errorf("Pattern(2) = %q, want %q", got, "baz")
	}
	if got := ac.Pattern(99); got != nil {
		t.Errorf("Pattern(99) = %v, want nil", got)
	}
	if got := ac.Pattern(-1); got != nil {
		t.Errorf("Pattern(-1) = %v, want nil", got)
	}

	// StateCount
	if got := ac.StateCount(); got <= 0 {
		t.Errorf("StateCount() = %d, want > 0", got)
	}

	// MatchKind
	if got := ac.MatchKind(); got != LeftmostFirst {
		t.Errorf("MatchKind() = %v, want LeftmostFirst", got)
	}
}

func TestMatchMethods(t *testing.T) {
	ac, _ := NewBuilder().
		AddStrings([]string{"hello"}).
		Build()

	match, found := ac.Find([]byte("say hello world"), 0)
	if !found {
		t.Fatal("expected match")
	}

	if match.Len() != 5 {
		t.Errorf("Len() = %d, want 5", match.Len())
	}
	if match.Start != 4 {
		t.Errorf("Start = %d, want 4", match.Start)
	}
	if match.End != 9 {
		t.Errorf("End = %d, want 9", match.End)
	}
}

func TestEmptyHaystack(t *testing.T) {
	ac, _ := NewBuilder().
		AddStrings([]string{"needle"}).
		Build()

	if ac.IsMatch([]byte{}) {
		t.Error("expected no match in empty haystack")
	}
	if _, found := ac.Find([]byte{}, 0); found {
		t.Error("expected no match in empty haystack")
	}
	if len(ac.FindAll([]byte{}, -1)) != 0 {
		t.Error("expected no matches in empty haystack")
	}
	if ac.Count([]byte{}) != 0 {
		t.Error("expected count 0 in empty haystack")
	}
}

func TestManyPatterns(t *testing.T) {
	// Build automaton with 1000 unique patterns (no prefix overlap)
	patterns := make([]string, 1000)
	for i := range patterns {
		// Use fixed-width numbering to avoid prefix overlap
		patterns[i] = "p" + padInt(i, 4) // p0000, p0001, ..., p0999
	}

	ac, err := NewBuilder().
		AddStrings(patterns).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	if ac.PatternCount() != 1000 {
		t.Errorf("PatternCount() = %d, want 1000", ac.PatternCount())
	}

	// Test finding a few patterns
	for _, i := range []int{0, 500, 999} {
		haystack := []byte("xxx" + patterns[i] + "yyy")
		match, found := ac.Find(haystack, 0)
		if !found {
			t.Errorf("pattern %d not found", i)
		} else if match.PatternID != i {
			t.Errorf("pattern %d: got ID %d", i, match.PatternID)
		}
	}
}

// padInt pads an integer with leading zeros to width w
func padInt(n, w int) string {
	s := itoa(n)
	for len(s) < w {
		s = "0" + s
	}
	return s
}

func TestByteClassesDisabled(t *testing.T) {
	ac, err := NewBuilder().
		SetByteClasses(false).
		AddStrings([]string{"abc", "xyz"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Should still work correctly
	if !ac.IsMatch([]byte("abc")) {
		t.Error("expected match for 'abc'")
	}
	if !ac.IsMatch([]byte("xyz")) {
		t.Error("expected match for 'xyz'")
	}
	if ac.IsMatch([]byte("def")) {
		t.Error("unexpected match for 'def'")
	}
}

func TestOverlappingPatternsDetailed(t *testing.T) {
	// The classic "ushers" test from Aho-Corasick paper
	ac, err := NewBuilder().
		AddStrings([]string{"he", "she", "his", "hers"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	haystack := []byte("ushers")
	matches := ac.FindAllOverlapping(haystack)

	// Expected matches:
	// - "she" at position 1-4
	// - "he" at position 2-4
	// - "hers" at position 2-6

	found := make(map[string]bool)
	for _, m := range matches {
		text := string(haystack[m.Start:m.End])
		found[text] = true
	}

	expected := []string{"she", "he", "hers"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected to find %q", e)
		}
	}
}

func TestRepeatedPattern(t *testing.T) {
	ac, err := NewBuilder().
		AddStrings([]string{"aa"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// "aaaa" should have 2 non-overlapping "aa" matches
	matches := ac.FindAll([]byte("aaaa"), -1)
	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2", len(matches))
	}

	// Check positions
	if len(matches) >= 2 {
		if matches[0].Start != 0 || matches[0].End != 2 {
			t.Errorf("first match: [%d:%d], want [0:2]", matches[0].Start, matches[0].End)
		}
		if matches[1].Start != 2 || matches[1].End != 4 {
			t.Errorf("second match: [%d:%d], want [2:4]", matches[1].Start, matches[1].End)
		}
	}
}

// Fuzz tests

func FuzzIsMatch(f *testing.F) {
	// Seed corpus
	f.Add([]byte("hello"), []byte("hello world"))
	f.Add([]byte("abc"), []byte("xyzabcdef"))
	f.Add([]byte("needle"), []byte("haystack"))

	f.Fuzz(func(t *testing.T, pattern, haystack []byte) {
		if len(pattern) == 0 {
			return
		}

		ac, err := NewBuilder().
			AddPattern(pattern).
			Build()
		if err != nil {
			return
		}

		// Our result
		got := ac.IsMatch(haystack)

		// Reference: bytes.Contains
		want := bytes.Contains(haystack, pattern)

		if got != want {
			t.Errorf("IsMatch mismatch for pattern %q in %q: got %v, want %v",
				pattern, haystack, got, want)
		}
	})
}

func FuzzFind(f *testing.F) {
	// Seed corpus
	f.Add([]byte("hello"), []byte("hello world"))
	f.Add([]byte("abc"), []byte("xyzabcdef"))

	f.Fuzz(func(t *testing.T, pattern, haystack []byte) {
		if len(pattern) == 0 {
			return
		}

		ac, err := NewBuilder().
			AddPattern(pattern).
			Build()
		if err != nil {
			return
		}

		match, found := ac.Find(haystack, 0)

		// Reference: bytes.Index
		idx := bytes.Index(haystack, pattern)

		// Compare results: check that our match position equals bytes.Index position
		switch {
		case idx < 0 && found:
			t.Errorf("Find found match at %d, but bytes.Index returned -1", match.Start)
		case idx >= 0 && !found:
			t.Errorf("Find returned no match, but bytes.Index found at %d", idx)
		case idx >= 0 && found && match.Start != idx:
			t.Errorf("Find returned %d, bytes.Index returned %d", match.Start, idx)
		}
	})
}

// Benchmarks

func BenchmarkFind(b *testing.B) {
	ac, _ := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal", "critical", "info", "debug", "trace"}).
		Build()

	haystack := make([]byte, 64*1024)
	for i := range haystack {
		haystack[i] = 'x'
	}
	copy(haystack[32*1024:], "error")

	b.ResetTimer()
	b.SetBytes(int64(len(haystack)))

	for i := 0; i < b.N; i++ {
		_, _ = ac.Find(haystack, 0)
	}
}

func BenchmarkIsMatch(b *testing.B) {
	ac, _ := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal", "critical"}).
		Build()

	haystack := make([]byte, 64*1024)
	for i := range haystack {
		haystack[i] = 'x'
	}
	// No match in haystack

	b.ResetTimer()
	b.SetBytes(int64(len(haystack)))

	for i := 0; i < b.N; i++ {
		_ = ac.IsMatch(haystack)
	}
}

func BenchmarkFindAll(b *testing.B) {
	ac, _ := NewBuilder().
		AddStrings([]string{"the", "and", "for", "are", "but", "not", "you", "all"}).
		Build()

	haystack := []byte("the quick brown fox and the lazy dog are not the same but you are all welcome")

	b.ResetTimer()
	b.SetBytes(int64(len(haystack)))

	for i := 0; i < b.N; i++ {
		_ = ac.FindAll(haystack, -1)
	}
}

// BenchmarkIsMatchNoMatch tests worst case - no match, scans entire haystack
func BenchmarkIsMatchNoMatch(b *testing.B) {
	ac, _ := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal", "critical"}).
		Build()

	haystack := make([]byte, 64*1024)
	for i := range haystack {
		haystack[i] = 'x'
	}
	// No match in haystack - worst case, scans all

	b.ResetTimer()
	b.SetBytes(int64(len(haystack)))

	for i := 0; i < b.N; i++ {
		_ = ac.IsMatch(haystack)
	}
}

// BenchmarkIsMatchWithMatch tests average case - match at 32KB
func BenchmarkIsMatchWithMatch(b *testing.B) {
	ac, _ := NewBuilder().
		AddStrings([]string{"error", "warning", "fatal", "critical"}).
		Build()

	haystack := make([]byte, 64*1024)
	for i := range haystack {
		haystack[i] = 'x'
	}
	copy(haystack[32*1024:], "error") // Match at 32KB

	b.ResetTimer()
	b.SetBytes(int64(len(haystack)))

	for i := 0; i < b.N; i++ {
		_ = ac.IsMatch(haystack)
	}
}

func BenchmarkCountLeftmostLongest(b *testing.B) {
	ac, err := NewBuilder().
		SetMatchKind(LeftmostLongest).
		AddStrings([]string{"a"}).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	haystack := bytes.Repeat([]byte("a"), 2048)
	b.SetBytes(int64(len(haystack)))

	for b.Loop() {
		_ = ac.Count(haystack)
	}
}
