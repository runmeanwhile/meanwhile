// Package ideo implements the IDEO-inspired brainstorming protocol.
//
// This protocol separates brainstorming into distinct sessions with deliberate
// context transfer between phases:
//
//   - Inspiration: Empathize, observe, gather tensions
//   - Reframe: Generate diverse HMW (How Might We) framings
//   - Ideation: Generate divergent concepts with artifact tools
//   - Synthesis: Converge to experiment-ready portfolio
//
// Each session has a distinct mindset and toolset. Context transfer is curated
// to prevent anchoring bias while preserving essential insights.
package ideo
