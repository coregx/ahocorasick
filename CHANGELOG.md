# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-08-05

Zero-allocation API release. Breaking change: `Find` and `FindAt` now return `(Match, bool)` instead of `*Match`.

### Changed

- **`Find` returns `(Match, bool)` instead of `*Match`**. Match is returned by value,
  eliminating the heap allocation. Throughput increased from 3.4 GB/s to 7.0 GB/s.
  Callers must update from `if m := ac.Find(h, 0); m != nil` to
  `if m, found := ac.Find(h, 0); found`.

- **`FindAt` returns `(Match, bool)` instead of `*Match`**. Same zero-allocation
  change as `Find`. Callers must update from `if m := ac.FindAt(h, 0); m != nil`
  to `if m, found := ac.FindAt(h, 0); found`.

### Added

- `AGENTS.md` — AI agent documentation for library users (API reference, common patterns, performance)
- `llms.txt` — LLM discovery file

### Performance

Benchmarks on Intel i7-1255U (64KB haystack, 4-7 patterns):

| Method | v0.2.1 | **v0.3.0** | Improvement |
|--------|--------|-----------|-------------|
| `Find` | 3.4 GB/s (1 alloc) | **7.0 GB/s (0 alloc)** | **2x, zero alloc** |
| `IsMatch` (match) | 7.0 GB/s | 7.0 GB/s | unchanged |
| `IsMatch` (no match) | 5.9 GB/s | 5.9 GB/s | unchanged |

## [0.2.1] - 2026-03-18

Major performance release: DFA compilation with SIMD-accelerated prefilter.

### Changed

- **DFA backend** replaces NFA for search. All failure transitions are pre-computed
  into a flat `[]uint32` transition table at build time, eliminating failure link
  following at search time entirely. Premultiplied state IDs allow single-instruction
  transitions: `trans[sid + byteClass]`.

- **Match flag in transition table**. High bit of each transition entry indicates
  whether the target state is a match state. Non-match bytes require zero masking
  in the hot loop — the raw value IS the clean state ID.

- **Inline DFA loop in FindAll**. Previously delegated to `Find()` per match,
  causing heap allocation for each `*Match`. Now uses a single DFA traversal
  with stack-allocated match values. Allocations reduced from 14 to 4 per call.

### Added

- **SIMD-accelerated start byte prefilter**. Before running the DFA, uses
  `bytes.IndexByte` (SIMD-optimized on amd64/arm64) to check if any pattern
  start byte exists in the haystack. If none found, returns immediately.

- **Skip-ahead prefilter inside search loop**. When the DFA returns to start
  state during search, re-engages the prefilter to skip ahead to the next
  position where a match could start. Avoids processing long runs of
  non-pattern bytes one at a time.

- `findEarliestStartByte` helper for prefilter position scanning.

### Performance

Benchmarks on Intel i7-1255U (64KB haystack, 4-7 patterns):

| Method | v0.1.0 | **v0.2.1** | Improvement |
|--------|--------|-----------|-------------|
| `Find` | 300 MB/s | **3.4 GB/s** | **11x** |
| `IsMatch` (no match) | 260 MB/s | **5.9 GB/s** | **23x** |
| `IsMatch` (match@32KB) | 545 MB/s | **7.0 GB/s** | **13x** |
| `FindAll` (77B, 10 matches) | 40 MB/s | **100 MB/s** | **2.5x** |

Memory: DFA uses a single flat array (~25KB for 100 states, stride 64).
Zero heap allocations for `IsMatch`.

## [0.1.0] - 2026-01-05

Initial release of the high-performance Aho-Corasick library for Go.

### Features

- **Builder pattern** for fluent automaton construction
  - `AddPattern`, `AddPatterns`, `AddStrings` for adding patterns
  - `SetMatchKind` for choosing match semantics
  - `SetByteClasses` for memory optimization control

- **Automaton API** for pattern matching
  - `Find` — first match from position
  - `FindAt` — match at exact position (anchored)
  - `FindAll` — all non-overlapping matches
  - `FindAllOverlapping` — all matches including overlaps
  - `IsMatch` — existence check (zero allocation)
  - `Count` — count non-overlapping matches

- **Match semantics**
  - `LeftmostFirst` — Perl-compatible (first pattern wins)
  - `LeftmostLongest` — POSIX-compatible (longest pattern wins)

- **Optimizations**
  - Dense array transitions for O(1) state lookup
  - Byte class compression (256 → N equivalence classes)
  - Precomputed root transitions (no failure link following for root)
  - Zero-allocation `IsMatch()` hot path

[Unreleased]: https://github.com/coregx/ahocorasick/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/coregx/ahocorasick/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/coregx/ahocorasick/compare/v0.1.0...v0.2.1
[0.1.0]: https://github.com/coregx/ahocorasick/releases/tag/v0.1.0
