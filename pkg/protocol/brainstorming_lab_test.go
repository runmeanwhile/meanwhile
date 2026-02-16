package protocol

import (
	"context"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestBrainstormingLabProtocolID(t *testing.T) {
	p := BrainstormingLab()
	if p.ID() != "protocol.brainstorming_lab" {
		t.Fatalf("unexpected ID: %s", p.ID())
	}
}

func TestBrainstormingLabOnMessageSuccess(t *testing.T) {
	plan := insightpack.DefaultPlan()
	plan.Strategy = insightpack.StrategyBalanced
	plan.Sources = []insightpack.Source{{
		ID:      "issues",
		Type:    insightpack.SourceInternal,
		ToolIDs: []string{"issues.search", "mem.search"},
	}}

	p := BrainstormingLab(
		WithBrainstormingLabScope("Improve weekly KPI meeting outcomes"),
		WithBrainstormingLabContextPlan(plan),
		WithBrainstormingLabDiscoveryRounds(1),
		WithBrainstormingLabChallengeRounds(1),
		WithBrainstormingLabInteractionRounds(1),
		WithBrainstormingLabCritiqueRounds(1),
		WithBrainstormingLabFrameTarget(4),
		WithBrainstormingLabFinalistCount(2),
	)

	var insightReq RunRequest
	sess := &mockSession{
		id: "lab-test",
		participants: []Participant{
			agent.Agent{Name: "Marketing"},
			agent.Agent{Name: "Engineering"},
			agent.Agent{Name: "Design"},
		},
		facilitator: &agent.Agent{Name: "Moderator"},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			_ = ctx
			system := ""
			if len(req.SystemMessages) > 0 {
				system = req.SystemMessages[0].Summary()
			}
			switch {
			case strings.Contains(system, "brainstorming moderator"):
				return message.Assistant("Improve KPI meeting follow-through without new workflow"), nil
			case strings.Contains(system, "INSIGHT INTAKE"):
				insightReq = req
				return message.Assistant("- Week-3 rerun effect | same numbers, no urgency | issues.search\n- Commitments decay | follow-through drops by week 4 | mem.search"), nil
			case strings.Contains(system, "BRAINSTORM LAB: REFRAME"):
				return message.Assistant("- operational | How might we force one urgent KPI opener each week? | reduces replay\n- behavioral | How might we make commitments visible by default? | raises ownership"), nil
			case strings.Contains(system, "DISCOVERY ROUND"):
				return message.Assistant("Question: Where exactly does accountability break?\nEvidence: Week-4 completion drops 18% [issues.search]\nSources: issues.search\nUncertainty: Unsure whether this is meeting fatigue or ownership diffusion."), nil
			case strings.Contains(system, "CHALLENGE ROUND"):
				return message.Assistant("I'm not confident the drop is only fatigue; role ambiguity could be a hidden driver [mem.search]. What evidence do we have by team role?"), nil
			case strings.Contains(system, "IDEATION SPRINT"):
				return message.Assistant("Building on Design's point, we should open each meeting with one risk-ranked KPI card. This keeps urgency visible and anchors commitments. Risk: poor data quality could undermine trust. Could we validate this with a two-week pilot first [issues.search]?"), nil
			case strings.Contains(system, "IDEA CRITIQUE"):
				return message.Assistant("Confidence: medium. This concept might still fail if ownership remains diffuse across teams. We need proof by role-level follow-through before scaling."), nil
			case strings.Contains(system, "EVIDENCE GATE"):
				return message.Assistant(`[{"title":"Urgency opener","concept":"Auto-open on highest risk KPI","core_assumption":"Urgent opener increases action","cheapest_test":"A/B one team","target_signal":"Task close rate","success_threshold":">=15%","failure_threshold":"<5%","time_to_learn":"2 weeks","risk_level":"medium","evidence_refs":"issues.search","confidence":"medium","unknowns":"role-level variance"}]`), nil
			case strings.Contains(system, "LAB CLOSING"):
				return message.Assistant("## Goal / Problem\nImprove KPI follow-through."), nil
			default:
				return message.Assistant("Concept: Use risk-triggered meeting opener with one explicit tradeoff."), nil
			}
		},
	}

	if err := p.Init(context.Background(), sess); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if err := p.OnMessage(context.Background(), sess, message.User("Find one feature to improve weekly KPI meetings.")); err != nil {
		t.Fatalf("OnMessage failed: %v", err)
	}

	if len(sess.emittedEvents) != 1 {
		t.Fatalf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}
	ev := sess.emittedEvents[0]
	if ev.Type != event.ProtocolAction {
		t.Fatalf("expected protocol action event, got %v", ev.Type)
	}
	payload, ok := ev.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", ev.Payload)
	}
	if _, ok := payload["insight_intake"]; !ok {
		t.Fatal("missing insight_intake in payload")
	}
	if _, ok := payload["reframing"]; !ok {
		t.Fatal("missing reframing in payload")
	}
	if _, ok := payload["discovery"]; !ok {
		t.Fatal("missing discovery in payload")
	}
	if _, ok := payload["portfolio"]; !ok {
		t.Fatal("missing portfolio in payload")
	}
	if _, ok := payload["evidence_gate"]; !ok {
		t.Fatal("missing evidence_gate in payload")
	}
	if len(insightReq.Tools) == 0 {
		t.Fatal("expected insight phase to include tools")
	}
	if insightReq.ToolPolicy.Mode != tool.PolicyAllowlist {
		t.Fatalf("expected allowlist tool policy, got %q", insightReq.ToolPolicy.Mode)
	}
}
