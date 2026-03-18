# ahocorasick

[![Go Reference](https://pkg.go.dev/badge/github.com/coregx/ahocorasick.svg)](https://pkg.go.dev/github.com/coregx/ahocorasick)
[![Tests](https://github.com/coregx/ahocorasick/actions/workflows/ci.yml/badge.svg)](https://github.com/coregx/ahocorasick/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/coregx/ahocorasick)](https://goreportcard.com/report/github.com/coregx/ahocorasick)
[![codecov](https://codecov.io/gh/coregx/ahocorasick/branch/main/graph/badge.svg)](https://codecov.io/gh/coregx/ahocorasick)
[![Release](https://img.shields.io/github/v/release/coregx/ahocorasick)](https://github.com/coregx/ahocorasick/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

High-performance Aho-Corasick multi-pattern string matching for Go.

## Features

- **Up to 7 GB/s throughput** — DFA compilation with SIMD-accelerated prefilter
- **Flat transition table** — fully compiled DFA with premultiplied state IDs
- **SIMD prefilter** — `bytes.IndexByte` skip-ahead for non-matching regions
- **Byte class compression** — reduces memory by grouping equivalent bytes
- **Multiple match semantics** — LeftmostFirst (Perl) and LeftmostLongest (POSIX)
- **Zero dependencies** — pure Go, no cgo
- **Zero allocations** — `IsMatch()` hot path allocates nothing

## Installation

```bash
go get github.com/coregx/ahocorasick
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/coregx/ahocorasick"
)

func main() {
    // Build automaton from patterns
    ac, _ := ahocorasick.NewBuilder().
        AddStrings([]string{"error", "warning", "fatal"}).
        Build()

    haystack := []byte("[error] something failed")

    // Check if any pattern matches (zero allocation)
    if ac.IsMatch(haystack) {
        fmt.Println("Found a match!")
    }

    // Find first match
    if m := ac.Find(haystack, 0); m != nil {
        fmt.Printf("Found %q at position %d\n",
            haystack[m.Start:m.End], m.Start)
    }

    // Find all non-overlapping matches
    for _, m := range ac.FindAll(haystack, -1) {
        fmt.Printf("Pattern %d: %q\n", m.PatternID, haystack[m.Start:m.End])
    }
}
```

## Performance

Benchmarks on Intel i7-1255U (64KB haystack, 4-7 patterns):

| Method | Throughput | Allocations |
|--------|------------|-------------|
| `IsMatch` (with match) | **7.0 GB/s** | 0 |
| `IsMatch` (no match) | **5.9 GB/s** | 0 |
| `Find` | **3.4 GB/s** | 1 |
| `FindAll` (77B input) | 100 MB/s | 4 |

### How it achieves this

1. **DFA compilation** — all failure transitions pre-computed at build time into a flat `[]uint32` array. Search is a single table lookup per byte: `trans[sid + class]`.
2. **SIMD prefilter** — before running the DFA, `bytes.IndexByte` (SSE2/AVX2 on amd64) scans for pattern start bytes. Skips entire regions where no match is possible.
3. **Skip-ahead on start state** — when the DFA returns to its start state during search, the prefilter re-engages to jump ahead, avoiding byte-by-byte scanning of non-matching text.
4. **Match flag embedding** — the high bit of each transition entry flags match states, enabling single-instruction match detection with no separate lookup.

## API

### Builder

```go
ahocorasick.NewBuilder().
    AddStrings([]string{"foo", "bar"}).  // Add patterns
    SetMatchKind(ahocorasick.LeftmostLongest).  // Optional: POSIX semantics
    Build()  // Returns (*Automaton, error)
```

### Automaton

```go
// Existence check (zero allocation)
ac.IsMatch(haystack []byte) bool

// Find matches
ac.Find(haystack []byte, start int) *Match      // First match from position
ac.FindAt(haystack []byte, start int) *Match    // Match at exact position
ac.FindAll(haystack []byte, n int) []Match      // All non-overlapping (n=-1 for all)
ac.FindAllOverlapping(haystack []byte) []Match  // All including overlaps

// Utilities
ac.Count(haystack []byte) int   // Count non-overlapping matches
ac.PatternCount() int           // Number of patterns
ac.Pattern(id int) []byte       // Get pattern by ID
```

### Match Semantics

```go
LeftmostFirst   // First pattern in list wins (Perl-compatible, default)
LeftmostLongest // Longest pattern wins (POSIX-compatible)
```

## Architecture

```
Builder.Build()
  -> NFA construction (trie + failure links)
  -> DFA compilation (flat transition table, premultiplied state IDs)
  -> Prefilter setup (start byte collection for SIMD scanning)

Search: IsMatch / Find / FindAll
  -> SIMD prefilter (bytes.IndexByte skip-ahead)
  -> DFA traversal (trans[sid + class], one operation per byte)
  -> Match flag check (raw & matchFlag, one AND per byte)
```

## Use Cases

- **Log analysis** — scan for error patterns in log files
- **Content filtering** — detect keywords in text
- **Network security** — signature-based detection
- **DNA sequencing** — find multiple motifs simultaneously
- **Regex acceleration** — as prefilter for `foo|bar|baz` alternations

## Articles

- [Aho-Corasick in Go: Multi-Pattern String Matching at 6 GB/s with Zero Allocations](https://dev.to/kolkov/aho-corasick-in-go-multi-pattern-string-matching-at-6-gbs-with-zero-allocations-2jog) — Deep dive into the DFA compilation, SIMD prefilter, and optimization techniques

## Related Projects

- [coregex](https://github.com/coregx/coregex) — High-performance regex engine (uses this library)
- [BurntSushi/aho-corasick](https://github.com/BurntSushi/aho-corasick) — Rust reference implementation

## License

MIT License — see [LICENSE](LICENSE) for details.
