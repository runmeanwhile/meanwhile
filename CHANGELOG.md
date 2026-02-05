# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Runtime
- Human-in-the-loop escalation (`ask_human`, human participants, and new examples).
- Outbound integrations (Slack, email, webhook) and inbound response handlers.
- Request registry drivers (in-memory + Redis) for mapping request IDs to sessions.
- Timeout scheduling with pluggable scheduler drivers (in-memory + Redis) and default timeout policies.

### Future Additions
- **CLI**: Command-line interface for session management (see [ROADMAP.md](ROADMAP.md))
- **Studio**: Web-based UI for visual collaboration (see [ROADMAP.md](ROADMAP.md))

## [0.1.0] - 2026-01-19

### Runtime

#### Core Framework
- **Engine**: Multi-agent session management with lifecycle control
- **Agent System**: Fluent builder API for agent configuration and registry
- **Session Management**: Support for single and multi-agent sessions
- **Event Bus**: Pub/sub system for agent and session events
- **Hook System**: Runtime control points for customizing agent behavior
- **Worklog**: Structured logging system for tracking agent activities

#### Protocol System
- **Solo Protocol**: Single-agent execution pattern
- **Handoff Protocol**: Agent-to-agent task delegation
- **Brainstorming Protocol**: Collaborative ideation among multiple agents
- **Adversarial Protocol**: Debate-style interaction between agents
- **Consensus Protocol**: Multi-agent decision-making with voting and moderation
- **Breakout Protocol**: Hierarchical agent spawning and management
- **Protocol-as-Tool**: Use protocols as executable tools within sessions
- **Custom Protocol Support**: Interface for implementing custom collaboration patterns

#### Provider System
- **OpenAI Provider**: Full support for OpenAI's chat completion API
- **Streaming**: Real-time response streaming with SSE
- **Tool Calling**: Native support for OpenAI function calling
- **Provider Registry**: Pluggable provider architecture
- **Provider Interface**: Extensible interface for additional LLM providers

#### Tool System
- **Tool Registry**: Dynamic tool registration and discovery
- **Typed Tools**: Type-safe tool implementations with generic support
- **Tool Builder**: Fluent API for creating tools
- **Protocol Tools**: Automatic tool generation from protocols
- **Helper Functions**: Utilities for common tool patterns

#### Memory System
- **Memory Interface**: Abstract memory storage interface
- **In-Memory Store**: Default implementation for session memory
- **Message History**: Conversation history management
- **Pluggable Storage**: Interface for custom memory backends

#### Configuration
- **Config Loader**: Environment-based configuration management
- **Provider Config**: API key and endpoint configuration
- **Validation**: Configuration validation and defaults

#### Observability
- **Telemetry Interface**: Pluggable observability system
- **Langfuse Integration**: Built-in Langfuse telemetry adapter
- **Event Tracking**: Automatic tracking of agent activities
- **Trace Correlation**: Request tracing across agent interactions

#### Documentation
- **README**: Comprehensive quickstart and feature overview
- **Architecture Guide** (`docs/architecture.md`): Framework design and patterns
- **Protocol Guide** (`docs/protocols.md`): Collaboration protocol documentation
- **Observability Guide** (`docs/observability.md`): Telemetry and monitoring
- **Package Documentation**: godoc comments for all public APIs
- **13 Progressive Examples**: From basic to full-stack implementations

#### Examples
1. **Single Agent**: Basic agent creation and execution
2. **Agent with Tools**: Tool integration and usage
3. **Two Agents Handoff**: Inter-agent task delegation
4. **Session with Result**: Structured result handling
5. **Protocol Brainstorming**: Collaborative ideation
6. **Protocol Consensus**: Multi-agent decision-making
7. **Protocol Adversarial**: Debate-style interactions
8. **Protocol Breakout**: Hierarchical agent management
9. **Protocol as Tool**: Using protocols within tools
10. **Memory Store**: Custom memory implementations
11. **Hooks Control**: Runtime behavior customization
12. **Custom Protocol**: Building custom collaboration patterns
13. **Full Stack**: Complete application example

#### Development Infrastructure
- **Makefile**: Build, test, and lint targets
- **golangci-lint**: Comprehensive linting configuration
- **Go Modules**: Proper dependency management
- **Test Suite**: 23 test files with good coverage

#### Open Source
- **MIT License**: Permissive open-source license
- **Contributing Guidelines**: Clear contribution process and standards
- **Code of Conduct**: Community behavior expectations
- **Security Policy**: Vulnerability disclosure process
- **CI/CD**: GitHub Actions for automated testing and linting
- **Dependabot**: Automated dependency updates

### Notes

This is the initial release of Meanwhile. The API is considered **unstable** and may change before 1.0.0. We will follow semantic versioning and document breaking changes in this changelog.

**Core packages** (agent, message, event) are relatively stable, while **evolving packages** (protocol signatures, engine options, provider interface) may see changes based on community feedback.

### Known Limitations

- Only OpenAI provider is currently implemented (more providers planned)
- Protocol test coverage needs improvement (targeted for 0.2.0)
- Examples are not fully tested (example test suite planned)
- Go 1.24+ required (may add backwards compatibility later)

### Migration Guide

This is the first release - no migration needed!

---

## Release Links

[Unreleased]: https://github.com/runmeanwhile/meanwhile/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/runmeanwhile/meanwhile/releases/tag/v0.1.0
