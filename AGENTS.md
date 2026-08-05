# AGENTS.md — ahocorasick

> High-performance Aho-Corasick multi-pattern string matching for Go. Up to 7 GB/s, zero allocations, pure Go.

## What is ahocorasick

`ahocorasick` searches for multiple string patterns simultaneously using the Aho-Corasick algorithm. It compiles patterns into a DFA (deterministic finite automaton) at build time, then scans input in a single pass — O(n) regardless of how many patterns. Designed for log scanning, content filtering, intrusion detection, and any workload where you need to match many keywords at once.

## Install

```bash
go get github.com/coregx/ahocorasick
```

Requires Go 1.25+. Zero external dependencies.

## Quick Start

```go
import "github.com/coregx/ahocorasick"

// Build automaton (do once, reuse across goroutines)
ac, err := ahocorasick.NewBuilder().
    AddStrings([]string{"error", "warning", "fatal"}).
    Build()
if err != nil {
    log.Fatal(err)
}

haystack := []byte("[error] disk full")

// Check existence (zero allocation)
if ac.IsMatch(haystack) {
    fmt.Println("matched")
}

// Find first match (zero allocation)
if m, found := ac.Find(haystack, 0); found {
    fmt.Printf("pattern %d at [%d:%d] = %q\n",
        m.PatternID, m.Start, m.End, haystack[m.Start:m.End])
}

// Find all non-overlapping matches
for _, m := range ac.FindAll(haystack, -1) {
    fmt.Printf("%q at %d\n", haystack[m.Start:m.End], m.Start)
}
```

## API Reference

### Builder

```go
ahocorasick.NewBuilder() *Builder

b.AddStrings([]string{...}) *Builder    // Add string patterns
b.AddPattern([]byte{...}) *Builder      // Add single byte pattern
b.AddPatterns([][]byte{...}) *Builder   // Add multiple byte patterns
b.SetMatchKind(kind MatchKind) *Builder // LeftmostFirst (default) or LeftmostLongest
b.SetByteClasses(bool) *Builder         // Alphabet compression (default: true)
b.SetPrefilter(bool) *Builder           // SIMD prefilter (default: true)
b.SetASCII(bool) *Builder               // ASCII-only optimization
b.Build() (*Automaton, error)           // Compile the automaton
```

### Automaton (all methods are goroutine-safe)

```go
// Existence — zero allocation
ac.IsMatch(haystack []byte) bool

// Single match — zero allocation, returns (Match, found)
ac.Find(haystack []byte, start int) (Match, bool)
ac.FindAt(haystack []byte, start int) (Match, bool)  // match at exact position

// Multiple matches — allocates result slice
ac.FindAll(haystack []byte, n int) []Match           // non-overlapping (n=-1 for all)
ac.FindAllOverlapping(haystack []byte) []Match       // including overlaps

// Utilities
ac.Count(haystack []byte) int      // count non-overlapping matches
ac.PatternCount() int              // number of compiled patterns
ac.Pattern(id int) []byte          // get pattern bytes by ID
ac.StateCount() int                // number of DFA states
ac.MatchKind() MatchKind           // configured match semantics
```

### Match

```go
type Match struct {
    PatternID int  // index of matched pattern (order of AddStrings/AddPattern)
    Start     int  // byte offset of match start
    End       int  // byte offset of match end (exclusive)
}
m.Len() int        // End - Start
```

### Match Semantics

| Kind | Behavior | Example: patterns `["a", "ab"]` in `"ab"` |
|------|----------|-------------------------------------------|
| `LeftmostFirst` (default) | First pattern in list wins | `"a"` at 0 |
| `LeftmostLongest` | Longest pattern wins | `"ab"` at 0 |

## Common Patterns

### Log scanning

```go
ac, _ := ahocorasick.NewBuilder().
    AddStrings([]string{"ERROR", "WARN", "FATAL", "PANIC"}).
    Build()

scanner := bufio.NewScanner(logFile)
for scanner.Scan() {
    if ac.IsMatch(scanner.Bytes()) {
        processAlert(scanner.Text())
    }
}
```

### Iterate matches from a position

```go
pos := 0
for pos < len(haystack) {
    m, found := ac.Find(haystack, pos)
    if !found {
        break
    }
    process(m)
    pos = m.End
}
```

### Content filtering with pattern lookup

```go
patterns := []string{"badword1", "badword2", "badword3"}
ac, _ := ahocorasick.NewBuilder().AddStrings(patterns).Build()

for _, m := range ac.FindAll(userInput, -1) {
    fmt.Printf("blocked: %q\n", patterns[m.PatternID])
}
```

### POSIX longest match

```go
ac, _ := ahocorasick.NewBuilder().
    SetMatchKind(ahocorasick.LeftmostLongest).
    AddStrings([]string{"go", "gopher", "golang"}).
    Build()

// Returns "golang" not "go"
m, _ := ac.Find([]byte("I love golang"), 0)
```

## Performance

| Method | Throughput | Allocations |
|--------|-----------|-------------|
| `IsMatch` | 6-7 GB/s | 0 |
| `Find` | 7 GB/s | 0 |
| `FindAll` (77B) | 200 MB/s | 4 (result slice) |

The automaton is immutable after `Build()` — safe for concurrent use without locks.

## Build & Test

```bash
go build ./...
go test ./...
go test -bench=. -benchmem ./...
```

## Links

- GitHub: https://github.com/coregx/ahocorasick
- Docs: https://pkg.go.dev/github.com/coregx/ahocorasick
- Organization: https://github.com/coregx
