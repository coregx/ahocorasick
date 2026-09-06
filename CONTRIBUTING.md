# Contributing to ahocorasick

Thank you for considering contributing to ahocorasick! This document outlines the development workflow and guidelines.

## Git Workflow (GitHub Flow)

This project uses GitHub Flow — a single `main` branch with feature branches merged via pull requests.

### Branch Structure

```
main                 # Production-ready code (tagged releases)
  ├─ feat/*          # New features
  ├─ fix/*           # Bug fixes
  ├─ perf/*          # Performance improvements
  └─ release/*       # Release preparation
```

### Workflow

1. **Create a feature branch** from `main`:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b feat/my-new-feature
   ```

2. **Work on your changes**, committing as you go:
   ```bash
   git add .
   git commit -m "feat: add my new feature"
   ```

3. **Push and create a pull request**:
   ```bash
   git push -u origin feat/my-new-feature
   gh pr create --title "feat: my new feature"
   ```

4. **Wait for CI** — all checks must pass (tests, lint, formatting on 3 OS).

5. **Squash merge** into `main` after review:
   ```bash
   gh pr merge --squash
   ```

Small fixes (typos, docs) can go directly to `main`.

## Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, etc.)
- **refactor**: Code refactoring
- **test**: Adding or updating tests
- **chore**: Maintenance tasks (build, dependencies, etc.)
- **perf**: Performance improvements

### Examples

```bash
feat: add Unicode property class support
fix: correct epsilon-closure in NFA compilation
docs: update README with performance benchmarks
refactor: simplify DFA state cache implementation
test: add fuzz tests for memchr edge cases
chore: update go.mod to Go 1.25.4
perf: optimize Teddy SIMD prefilter for AVX2
```

## Code Quality Standards

### Before Committing

1. **Format code**:
   ```bash
   go fmt ./...
   ```

2. **Run linter**:
   ```bash
   golangci-lint run
   ```

3. **Run tests**:
   ```bash
   go test ./...
   ```

4. **Run tests with race detector**:
   ```bash
   go test -race ./...
   ```

5. **All-in-one** (use pre-release script):
   ```bash
   bash scripts/pre-release-check.sh
   ```

### Pull Request Requirements

- [ ] Code is formatted (`go fmt ./...`)
- [ ] Linter passes (`golangci-lint run` - 0 issues)
- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test -race ./...`)
- [ ] New code has tests (minimum 70% coverage)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] No sensitive data (credentials, tokens, etc.)
- [ ] Benchmarks added for performance-critical code

## Development Setup

### Prerequisites

- Go 1.25 or later
- golangci-lint
- GCC or Clang (for race detector)
- Optional: WSL2 with Go (for Windows users without GCC)

### Install Dependencies

```bash
# Clone repository
git clone https://github.com/coregx/ahocorasick.git
cd ahocorasick

# Download dependencies
go mod download

# Install golangci-lint
# See: https://golangci-lint.run/welcome/install/
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector (requires CGO)
CGO_ENABLED=1 go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...
```

### Running Linter

```bash
# Run linter
golangci-lint run

# Run with verbose output
golangci-lint run -v
```

## Project Structure

```
ahocorasick/
├── .github/              # GitHub workflows and templates
│   ├── CODEOWNERS       # Code ownership
│   └── workflows/       # CI/CD pipelines
├── ahocorasick.go        # Package doc and version constant
├── automaton.go          # Search API (Find, FindAt, FindAll, IsMatch, Count)
├── builder.go            # Builder pattern for configuration
├── byteclasses.go        # Alphabet compression (256 → N equivalence classes)
├── dfa.go                # DFA compilation (flat transition table)
├── match.go              # Match, MatchKind, PatternID, StateID types
├── nfa.go                # NFA construction (trie + failure links)
├── *_test.go             # Tests, benchmarks, fuzz tests
├── .golangci.yml         # Linter configuration
├── AGENTS.md             # AI agent documentation
├── CHANGELOG.md          # Version history
├── CONTRIBUTING.md       # This file
├── LICENSE               # MIT License
├── README.md             # Main documentation
└── llms.txt              # LLM discovery file
```

## Adding New Features

1. Check if issue exists, if not create one
2. Discuss approach in the issue
3. Create feature branch from `main`
4. Implement feature with tests
5. Update documentation
6. Run quality checks (build, test, lint, format)
7. Create pull request to `main`
8. Wait for CI and code review
9. Address feedback
10. Squash merge when approved

## Code Style Guidelines

### General Principles

- Follow Go conventions and idioms
- Write self-documenting code
- Add comments for complex logic (especially SIMD/assembly code)
- Keep functions small and focused
- Use meaningful variable names
- Optimize for clarity first, performance second (except in hot paths)

### Naming Conventions

- **Public types/functions**: `PascalCase` (e.g., `Compile`, `Memchr`)
- **Private types/functions**: `camelCase` (e.g., `epsilonClosure`, `selectPrefilter`)
- **Constants**: `PascalCase` with context prefix (e.g., `UseNFA`, `UseDFA`)
- **Test functions**: `Test*` (e.g., `TestLazyDFAFind`)
- **Benchmark functions**: `Benchmark*` (e.g., `BenchmarkMemchr`)
- **Example functions**: `Example*` (e.g., `ExampleCompile`)

### Error Handling

- Always check and handle errors
- Use descriptive error variables (`ErrCacheFull`, `ErrInvalidPattern`)
- Return errors immediately, don't wrap unnecessarily
- Validate inputs before processing

### Testing

- Use table-driven tests when appropriate
- Test both success and error cases
- Include test data in `testdata/` if needed
- Test edge cases (empty input, boundaries, alignment)
- Add fuzz tests for parsers and matchers
- Compare with stdlib regexp for correctness
- **Benchmarks are mandatory** for performance-critical code

## Aho-Corasick Implementation Guidelines

### Core Algorithm

- Trie construction followed by failure link computation
- BFS traversal for failure links (ensures correct order)
- Match propagation along failure links for overlapping patterns
- O(n + m) time complexity (n = text length, m = total pattern length)

### Memory Efficiency

- Use ByteClasses for alphabet compression
- Consider contiguous state layout for cache efficiency
- Minimize allocations in hot paths

### Performance Validation

```go
// Benchmarks should include:
// - Various input sizes (1KB, 64KB, 1MB)
// - Different pattern counts (4, 16, 64, 256)
// - Memory allocation tracking
// - Throughput calculation (MB/s)

func BenchmarkFind(b *testing.B) {
    ac, _ := NewBuilder().
        AddStrings(patterns).
        Build()

    b.SetBytes(int64(len(haystack)))
    for i := 0; i < b.N; i++ {
        _, _ = ac.Find(haystack, 0)
    }
}
```

## Getting Help

- Check existing issues and discussions
- Ask questions in GitHub Issues
- See [coregex](https://github.com/coregx/coregex) for integration examples

## Performance Expectations

- Multi-pattern matching: O(n) regardless of pattern count
- Throughput: 100+ MB/s for typical workloads
- Zero allocations in IsMatch hot path
- Comparable to Rust's aho-corasick crate

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to ahocorasick!**
