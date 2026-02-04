package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

// RoundRobinProtocol rotates through participants sequentially
type RoundRobinProtocol struct {
	maxRounds int
	round     int
}

func NewRoundRobin(maxRounds int) protocol.Protocol {
	return &RoundRobinProtocol{maxRounds: maxRounds}
}

func (p *RoundRobinProtocol) ID() string { return "custom.round_robin" }

func (p *RoundRobinProtocol) Participants() []protocol.Participant { return nil }

func (p *RoundRobinProtocol) Init(ctx context.Context, sess protocol.Session) error {
	p.round = 0
	return nil
}

func (p *RoundRobinProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return fmt.Errorf("round robin requires participants")
	}

	// Process each participant in sequence
	currentMsg := msg
	for p.round < p.maxRounds {
		agentIdx := p.round % len(participants)
		currentParticipant := participants[agentIdx]
		currentAgent, ok := currentParticipant.Agent()
		if !ok {
			return fmt.Errorf("round robin requires agent participants")
		}

		resp, err := sess.RunAgent(ctx, currentAgent, protocol.RunRequest{
			Messages: []agent.Message{currentMsg},
		})
		if err != nil {
			return err
		}

		// Emit progress
		payload := map[string]any{
			"round":   p.round + 1,
			"agent":   currentParticipant.DisplayName(),
			"message": resp,
		}
		if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
			return err
		}

		// Next agent responds to this agent's output
		currentMsg = resp
		p.round++
	}

	return nil
}

func (p *RoundRobinProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	return nil
}

func (p *RoundRobinProtocol) Shutdown(ctx context.Context, sess protocol.Session) error {
	return nil
}

func main() {
	ctx := context.Background()

	provider, _ := openai.FromEnv()
	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	// Chain of agents for a ticket escalation workflow
	tier1 := eng.Agent("Tier 1").
		Prompt("You're tier 1 support. Assess the issue and pass to tier 2 with notes. Keep it brief—they'll read your notes while the user is waiting.").
		Model("gpt-4o-mini").
		Build()

	tier2 := eng.Agent("Tier 2").
		Prompt("You're tier 2. You got notes from tier 1. Investigate deeper and pass to tier 3 if needed. You're the filter between basic issues and expert time.").
		Model("gpt-4o-mini").
		Build()

	tier3 := eng.Agent("Tier 3").
		Prompt("You're tier 3. You're the expert. You got context from tier 1 and tier 2. Provide the definitive solution. You're expensive.").
		Model("gpt-4o-mini").
		Build()

	// Use custom round-robin protocol
	sess, _ := eng.Session("Escalation Chain").
		Participants(tier1, tier2, tier3).
		Protocol(NewRoundRobin(3)).
		Start(ctx)

	result, err := eng.Run(ctx, sess.ID(), message.User("User reports: 'Application freezes when I click Save after editing records with special characters in the name field'"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Final Diagnosis ===")
	fmt.Println(result.Final)
}
