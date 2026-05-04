// Package protocol defines collaboration protocol interfaces and registry.
//
// Protocols are the heart of Meanwhile—they define how agents collaborate.
// Think of protocols as meeting styles: brainstorming sessions, debates,
// consensus-building, delegation handoffs, or breakout groups.
//
// Built-in protocols:
//
//   - Solo         - Single-agent execution (default)
//   - Handoff      - Simple delegation from one agent to another
//   - Brainstorming - Diverge, interact, and vote like a real brainstorm
//   - Adversarial  - Debate with opposing positions
//   - Consensus    - Convergent collaboration to agreement
//   - Breakout     - Parallel group work with synthesis
//   - Caucus       - Private per-participant prep
//
// All protocols support functional options for customization:
//
//	proto := protocol.Brainstorming(
//	    protocol.WithBrainstormingConcurrency(3),
//	)
//
// Protocols can be wrapped as tools via engine.AsTool() for nested collaboration.
//
// Meanwhile... protocols orchestrate, agents collaborate, and work gets done.
package protocol
