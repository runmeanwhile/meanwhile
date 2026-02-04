// Package consensus implements a structured multi-agent consensus protocol.
//
// The protocol enables agents to reach agreement through round-robin discussion
// with explicit position signaling (agree, conditional, block, abstain).
//
// A facilitator/chair intervenes at configured checkpoints to encourage
// convergence and surface blockers.
//
// Example:
//
//	p := consensus.Consensus(
//	    consensus.WithMaxRounds(10),
//	    consensus.WithAgenda(agenda.WithScope("Define API authentication approach")),
//	    consensus.WithChair(chair.WithInterventions(0.5, 0.8, 0.9)),
//	)
package consensus
