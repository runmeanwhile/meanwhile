# Contributing to Meanwhile

Thank you for your interest in contributing to Meanwhile! This framework for multi-agent collaboration is designed with specific architectural principles in mind, and we welcome contributions that align with these goals.

## Philosophy

Meanwhile is built on carefully considered design patterns and architectural decisions. While we welcome community contributions, we prioritize **quality over quantity** and **architectural coherence over feature abundance**. This is not a free-for-all for random features – it's a curated framework with a clear vision.

### What We Value

- **Architectural integrity**: Contributions that align with and strengthen the existing design patterns
- **Core improvements**: Enhancements to the engine, protocol system, and fundamental abstractions
- **Well-tested code**: Comprehensive tests for all new functionality
- **Clear documentation**: Both code documentation and user-facing guides
- **Performance**: Efficient implementations that don't compromise the framework's performance
- **Idiomatic Go**: Clean, idiomatic Go code following community standards

### What We're Cautious About

- **Feature creep**: Adding complexity without clear, substantial value
- **Breaking changes**: Modifications that would disrupt existing users
- **Architectural drift**: Contributions that diverge from the core design principles
- **Poorly tested code**: Features without adequate test coverage
- **Niche use cases**: Features that serve very specific needs rather than broad applicability

## Before You Start

1. **Check existing issues**: Look for related discussions or planned work
2. **Open an issue first**: For significant changes, discuss your proposal before coding
3. **Understand the architecture**: Read the [architecture documentation](docs/architecture.md)
4. **Review the code**: Familiarize yourself with the existing patterns and conventions

## Development Setup

### Prerequisites

- Go 1.24 or later
- Git
- Make
- golangci-lint (for linting)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/darkostanimirovic/meanwhile.git
cd meanwhile

# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint

# Run all checks
make
```

## Types of Contributions

### 🎯 High-Priority Contributions

These are always welcome:

- **Bug fixes**: Especially with tests that demonstrate the issue
- **Test coverage**: Improving test coverage in core packages (engine, protocol, provider)
- **Documentation**: Clarifying existing features, adding examples
- **Performance improvements**: With benchmarks showing the impact
- **Provider implementations**: Additional LLM provider support (Anthropic, local models, etc.)
- **Protocol implementations**: New collaboration patterns that align with the framework's philosophy

### ⚠️ Discussion-Required Contributions

These need upfront discussion via GitHub issues:

- **New core features**: Changes to the fundamental framework behavior
- **API changes**: Modifications to public interfaces
- **New packages**: Adding top-level packages to the framework
- **Breaking changes**: Anything that would require users to update their code
- **Architectural changes**: Modifications to the design patterns

### 📝 Documentation Contributions

Documentation improvements are always welcome:

- Fixing typos or unclear explanations
- Adding code examples
- Improving API documentation
- Creating guides for common use cases
- Translating documentation (if we expand internationally)

## Contribution Process

### 1. Fork and Branch

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR-USERNAME/meanwhile.git
cd meanwhile

# Add upstream remote
git remote add upstream https://github.com/darkostanimirovic/meanwhile.git

# Create a feature branch
git checkout -b feature/my-contribution
```

## Developer Certificate of Origin (DCO)

By contributing to this project, you certify that your contributions are made under the Apache 2.0 license and that you have the right to submit them.

Please sign off your commits:

```bash
git commit -s -m "Your commit message"
```

### 2. Make Your Changes

- Write clean, idiomatic Go code
- Follow existing code style and patterns
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

## Changelog Updates

- Add entries under the correct component heading in `CHANGELOG.md` (Runtime, CLI, Studio, Studio Enterprise).
- If a change is enterprise-only, call it out explicitly.

### 3. Test Your Changes

```bash
# Run all tests
make test

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run linter
make lint

# Ensure your code builds
go build ./...
```

### 4. Commit Your Changes

Write clear, descriptive commit messages:

```
Short summary (50 chars or less)

More detailed explanation if needed. Wrap at 72 characters.
Explain what changed and why, not how.

- Bullet points are fine
- Reference issues: Fixes #123
```

### 5. Submit a Pull Request

- Push your branch to your fork
- Open a pull request against `main`
- Fill out the PR template completely
- Link to any related issues
- Be responsive to feedback

## Code Standards

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for formatting
- Use `golangci-lint` for linting

### Testing Requirements

- **Core packages**: Minimum 85% coverage (engine, protocol, provider)
- **Supporting packages**: Minimum 70% coverage
- **All new features**: Must include tests
- **Bug fixes**: Must include regression tests
- **Test style**: Table-driven tests where appropriate

### Documentation Requirements

- **All exported types**: Must have godoc comments
- **All exported functions**: Must have godoc comments
- **Packages**: Must have doc.go file with package overview
- **Complex logic**: Inline comments explaining the "why"
- **Examples**: Include runnable examples for key features

### Error Handling

- Return errors, don't panic (except for programmer errors)
- Wrap errors with context using `fmt.Errorf`
- Define custom error types for distinct error conditions
- Document expected errors in function comments

## Pull Request Review Process

### What We Look For

1. **Does it align with the architecture?** Core design principles must be preserved
2. **Is it well-tested?** Adequate test coverage with meaningful tests
3. **Is it documented?** Clear godoc comments and updated guides if needed
4. **Is it performant?** No obvious performance regressions
5. **Is it maintainable?** Code that will be clear to future contributors

### Review Timeline

- **Initial response**: Within 3 business days
- **Full review**: Within 1 week for small PRs, 2 weeks for larger changes
- **Feedback iteration**: We'll work with you to get your PR ready

### After Acceptance

- Your PR will be squash-merged to maintain clean history
- You'll be credited in the CHANGELOG
- Breaking changes wait for the next minor/major version

## Community Guidelines

- Be respectful and professional
- Assume good intentions
- Accept constructive feedback gracefully
- Help others when you can
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md)

## Questions?

- **General questions**: Open a GitHub Discussion
- **Bug reports**: Open an issue with reproduction steps
- **Feature proposals**: Open an issue for discussion first
- **Security issues**: See [SECURITY.md](SECURITY.md)

## Recognition

Contributors who make significant, high-quality contributions will be recognized in:
- The CHANGELOG for each release
- A CONTRIBUTORS file (if the project grows)
- Release notes for major features

Thank you for helping make Meanwhile better! Your contributions, whether large or small, help advance the state of multi-agent collaboration frameworks.

---

**Note**: These guidelines may evolve as the project matures. The maintainers reserve the right to make final decisions on contributions to preserve the framework's vision and quality.
