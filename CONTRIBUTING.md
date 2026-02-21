# Contributing to GoSonata

Thank you for your interest in contributing to GoSonata! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Process](#development-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Release Process](#release-process)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please be respectful and professional in all interactions.

## Getting Started

### Prerequisites

- Go 1.26 or later
- Git
- [Task](https://taskfile.dev) (optional, but recommended)
- Basic understanding of JSONata

### Setup Development Environment

1. Fork the repository on GitHub

2. Clone your fork:

   ```bash
   git clone https://github.com/YOUR_USERNAME/gosonata.git
   cd gosonata
   ```

3. Add upstream remote:

   ```bash
   git remote add upstream https://github.com/sandrolain/gosonata.git
   ```

4. Install dependencies:

   ```bash
   task install
   # or
   go mod download
   ```

5. Verify setup:

   ```bash
   task test
   task lint
   ```

## Development Process

### Branching Strategy

- `main` - Stable releases
- `develop` - Integration branch for features
- `feature/*` - New features
- `fix/*` - Bug fixes
- `docs/*` - Documentation updates

### Workflow

1. Create a feature branch:

   ```bash
   git checkout -b feature/my-feature develop
   ```

2. Make your changes following our [Coding Standards](#coding-standards)

3. Write tests for your changes

4. Run tests and linters:

   ```bash
   task test
   task lint
   ```

5. Commit your changes:

   ```bash
   git commit -m "feat: add new feature"
   ```

6. Push to your fork:

   ```bash
   git push origin feature/my-feature
   ```

7. Create a Pull Request on GitHub

### Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types**:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks
- `ci`: CI/CD changes

**Examples**:

```
feat(parser): add support for wildcard operator
fix(evaluator): correct sequence flattening logic
docs: update API examples in README
test(conformance): add tests for string functions
perf(parser): optimize token scanning
```

## Coding Standards

### Go Guidelines

Follow the guidelines in [.github/copilot-instructions.md](.github/copilot-instructions.md):

1. **Style**: Follow [Effective Go](https://go.dev/doc/effective_go)
2. **Formatting**: Use `gofmt` - run `task fmt`
3. **Naming**: Use clear, idiomatic Go names
4. **Errors**: Always handle errors explicitly
5. **Context**: Use `context.Context` for long-running operations
6. **Documentation**: Document all exported types and functions

### Code Organization

```
pkg/           # Public packages
internal/      # Private implementation
cmd/           # Command-line tools
tests/         # Test suites
```

### Example Code Style

```go
// Good: Clear function with documentation
// Sum calculates the sum of all numbers in the array.
//
// Returns 0 for empty arrays. Returns an error if any element
// cannot be converted to a number.
func Sum(ctx context.Context, args []interface{}) (float64, error) {
    if len(args) == 0 {
        return 0, nil
    }

    var total float64
    for _, arg := range args {
        num, err := toNumber(arg)
        if err != nil {
            return 0, fmt.Errorf("cannot convert to number: %w", err)
        }
        total += num
    }

    return total, nil
}
```

### Documentation

- Use GoDoc format for all exported identifiers
- Include examples in documentation
- Keep comments up-to-date with code changes
- Explain "why" not just "what" for complex logic

## Testing

### Test Requirements

- **Coverage**: Minimum 90% code coverage
- **Types**: Unit, integration, conformance, benchmark tests
- **Quality**: Tests should be clear, maintainable, and fast

### Writing Tests

```go
func TestSum(t *testing.T) {
    tests := []struct {
        name    string
        args    []interface{}
        want    float64
        wantErr bool
    }{
        {
            name: "sum of numbers",
            args: []interface{}{1.0, 2.0, 3.0},
            want: 6.0,
        },
        {
            name: "empty array",
            args: []interface{}{},
            want: 0,
        },
        {
            name:    "invalid type",
            args:    []interface{}{"not a number"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Sum(context.Background(), tt.args)
            if (err != nil) != tt.wantErr {
                t.Errorf("Sum() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Sum() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Running Tests

```bash
# All tests
task test

# Specific tests
task test:unit
task test:integration
task test:conformance

# With coverage
task coverage

# Watch mode
task test:watch
```

### Benchmarks

Add benchmarks for performance-critical code:

```go
func BenchmarkSum(b *testing.B) {
    args := []interface{}{1.0, 2.0, 3.0, 4.0, 5.0}
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = Sum(ctx, args)
    }
}
```

Run benchmarks:

```bash
task bench
```

## Submitting Changes

### Before Submitting

1. ✅ Tests pass: `task test`
2. ✅ Linters pass: `task lint`
3. ✅ Coverage meets threshold: `task coverage:check`
4. ✅ Security scans pass: `task security`
5. ✅ Documentation updated
6. ✅ Commit messages follow convention

### Pull Request Process

1. **Title**: Use conventional commit format

   ```
   feat(parser): add wildcard operator support
   ```

2. **Description**: Provide context and details

   ```markdown
   ## Changes
   - Added wildcard operator parsing
   - Updated AST node types
   - Added tests for wildcard behavior

   ## Related Issues
   Closes #123

   ## Testing
   - Added unit tests
   - Verified with conformance tests
   - Benchmarked performance impact
   ```

3. **Review**: Address review comments promptly

4. **Updates**: Keep PR updated with develop branch

   ```bash
   git fetch upstream
   git rebase upstream/develop
   ```

### Review Criteria

We review for:

- ✅ Code quality and style
- ✅ Test coverage and quality
- ✅ Documentation completeness
- ✅ Performance impact
- ✅ Security considerations
- ✅ Breaking changes (if any)

## Areas for Contribution

### High Priority

- **Streaming API**: `EvalStream` for large JSON documents
- **Expression caching**: Implement `WithCaching` option
- **Custom function registration**: Complete `FunctionRegistry` integration
- **Performance**: Profiling and hot-path optimizations
- **Fuzzing**: Fuzz tests for parser and evaluator
- **Test Suite**: Additional edge-case conformance tests

> **Note**: Core implementation (lexer, parser, evaluator, all 66+ built-in functions)
> is complete — 1273/1273 official JSONata test suite cases (102 groups) + 249 imported conformance tests, all passing.

### Documentation

- API examples
- Tutorials and guides
- Performance guides
- Migration guides

### Testing

- Additional test cases
- Performance benchmarks
- Fuzzing tests
- Edge case coverage

### Performance

- Optimization opportunities
- Memory reduction
- Concurrency improvements

## Release Process

Releases follow semantic versioning (semver):

- **v0.x.x**: Development releases
- **v1.0.0**: First stable release
- **v1.x.x**: Minor releases (features, non-breaking)
- **vX.0.0**: Major releases (breaking changes)

## Getting Help

- 📖 Read the [documentation](https://godoc.org/github.com/sandrolain/gosonata)
- 💬 Ask in [Discussions](https://github.com/sandrolain/gosonata/discussions)
- 🐛 Report bugs in [Issues](https://github.com/sandrolain/gosonata/issues)
- 📧 Contact: [email or contact method]

## Recognition

Contributors will be recognized in:

- README.md contributors section
- Release notes
- GitHub contributors page

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to GoSonata! 🎉
