package interrupts

import (
	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
)

// Interrupt is a collaboration-kit alias of hook.Interrupt.
type Interrupt = hook.Interrupt

// Messages extracts agent messages from interrupts.
func Messages(interrupts []Interrupt) []agent.Message {
	out := make([]agent.Message, 0, len(interrupts))
	for _, interrupt := range interrupts {
		out = append(out, interrupt.Message)
	}
	return out
}
