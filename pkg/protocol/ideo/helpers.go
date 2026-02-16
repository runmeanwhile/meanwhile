package ideo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Re-export from protocol package for convenience within ideo package
type (
	ToolFinding    = protocol.ToolFinding
	SharedFindings = protocol.SharedFindings
)

// NewSharedFindings creates a new findings tracker.
var NewSharedFindings = protocol.NewSharedFindings

// ExtractFindingsFromResponse parses an agent's response for tool findings.
var ExtractFindingsFromResponse = protocol.ExtractFindingsFromResponse

// formatToolsSection formats the plan's sources into a tools section for prompts.
func formatToolsSection(plan insightpack.Plan) string {
	if len(plan.Sources) == 0 {
		return "TOOLS: No specific tools configured. Use any available tools to gather evidence."
	}

	var sb strings.Builder
	sb.WriteString("AVAILABLE TOOLS:\n")
	for _, src := range plan.Sources {
		if len(src.ToolIDs) == 0 {
			continue
		}
		desc := strings.TrimSpace(src.Description)
		if desc == "" {
			desc = string(src.Type) + " source"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", strings.Join(src.ToolIDs, ", "), desc))
	}

	if sb.Len() == len("AVAILABLE TOOLS:\n") {
		return "TOOLS: No specific tools configured. Use any available tools to gather evidence."
	}

	sb.WriteString("\nUse these tools to find concrete evidence. Cite sources when sharing findings.")
	return sb.String()
}

// rotateAgentOrder rotates agent order based on round number.
func rotateAgentOrder(agents []agent.Agent, round int) []agent.Agent {
	if len(agents) == 0 {
		return agents
	}
	offset := (round - 1) % len(agents)
	result := make([]agent.Agent, len(agents))
	for i := range agents {
		result[i] = agents[(i+offset)%len(agents)]
	}
	return result
}

// recentThread returns the most recent messages from a thread.
func recentThread(thread []agent.Message, n int) []agent.Message {
	if len(thread) <= n {
		return append([]agent.Message(nil), thread...)
	}
	return append([]agent.Message(nil), thread[len(thread)-n:]...)
}

// recentPeer finds the most recent speaker other than the excluded name.
func recentPeer(thread []agent.Message, exclude string) string {
	for i := len(thread) - 1; i >= 0; i-- {
		name := strings.TrimSpace(thread[i].Name)
		if name == "" || strings.EqualFold(name, strings.TrimSpace(exclude)) {
			continue
		}
		return name
	}
	return ""
}

// selectNudge picks a nudge based on round number.
func selectNudge(nudges []string, round int) string {
	if len(nudges) == 0 {
		return ""
	}
	return nudges[(round-1)%len(nudges)]
}

// truncate shortens a string to max length.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// deduplicateStrings removes duplicates from a slice.
func deduplicateStrings(items []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PromptWithMedia creates a message with any non-text content parts from the source.
func PromptWithMedia(text string, source agent.Message) agent.Message {
	msg := agent.Message{Role: agent.RoleUser}
	if text != "" {
		msg.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: text}}
	}
	if len(source.Parts) == 0 {
		return msg
	}

	// Filter non-text parts (media)
	mediaParts := make([]agent.ContentPart, 0)
	for _, part := range source.Parts {
		if part.Type != agent.ContentPartText && part.Type != "input_text" {
			mediaParts = append(mediaParts, part)
		}
	}
	if len(mediaParts) == 0 {
		return msg
	}

	parts := make([]agent.ContentPart, 0, len(mediaParts)+1)
	if text != "" {
		parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
	}
	parts = append(parts, mediaParts...)
	msg.Parts = parts

	return msg
}

// registerArtifactTools registers the sketch tools for ideation.
func (p *brainstormIDEO) registerArtifactTools(sess protocol.Session) error {
	// Sketch diagram tool
	diagramTool, err := tool.New("sketch_diagram", func(_ context.Context, input DiagramInput) (DiagramOutput, error) {
		return DiagramOutput{
			Status: "created",
			Artifact: Artifact{
				Type:    "mermaid",
				Title:   input.Title,
				Content: input.Content,
				Context: input.Context,
			},
		}, nil
	})
	if err != nil {
		return fmt.Errorf("create diagram tool: %w", err)
	}
	diagramTool = diagramTool.WithDescription(`Create a mermaid diagram to visualize a concept.

Use this to sketch:
- User flows and journeys
- System architectures
- Decision trees
- Process flows
- Relationship maps

Input a title, mermaid diagram content, and optional context explaining why you created this diagram.`)

	// Sketch concept card tool
	cardTool, err := tool.New("sketch_concept_card", func(_ context.Context, input ConceptCardInput) (ConceptCardOutput, error) {
		return ConceptCardOutput{
			Status: "created",
			Card: ConceptCard{
				Title:     input.Title,
				Problem:   input.Problem,
				Mechanism: input.Mechanism,
				Value:     input.Value,
				Risk:      input.Risk,
			},
		}, nil
	})
	if err != nil {
		return fmt.Errorf("create card tool: %w", err)
	}
	cardTool = cardTool.WithDescription(`Create a structured concept card.

Use this to capture an idea with:
- Title: Brief name for the concept
- Problem: What user problem does this solve?
- Mechanism: How does it work?
- Value: What's the benefit?
- Risk: What could go wrong?

Forces you to think through the idea concretely.`)

	// Sketch journey tool
	journeyTool, err := tool.New("sketch_journey", func(_ context.Context, input JourneyInput) (JourneyOutput, error) {
		stages := make([]JourneyStage, len(input.Stages))
		for i, s := range input.Stages {
			stages[i] = JourneyStage{
				Name:       s.Name,
				UserAction: s.UserAction,
				Emotion:    s.Emotion,
				Touchpoint: s.Touchpoint,
			}
		}
		return JourneyOutput{
			Status: "created",
			Journey: Journey{
				Title:  input.Title,
				Stages: stages,
			},
		}, nil
	})
	if err != nil {
		return fmt.Errorf("create journey tool: %w", err)
	}
	journeyTool = journeyTool.WithDescription(`Create a user journey map.

Use this to map out user experience over time:
- Title: Name for this journey
- Stages: List of stages, each with:
  - Name: Stage name (e.g., "Day 1")
  - User Action: What the user does
  - Emotion: How they feel (curious, confused, satisfied, etc.)
  - Touchpoint: Where this happens (app, email, etc.)

Helps visualize the emotional arc of an experience.`)

	if err := sess.RegisterTools(diagramTool, cardTool, journeyTool); err != nil {
		return err
	}

	return nil
}

// Tool input/output types

// DiagramInput is the input for the sketch_diagram tool.
type DiagramInput struct {
	Title   string `json:"title" description:"Brief title for the diagram"`
	Content string `json:"content" description:"Mermaid diagram content (e.g., 'graph LR; A-->B')"`
	Context string `json:"context,omitempty" description:"Why you created this diagram"`
}

// DiagramOutput is the output from the sketch_diagram tool.
type DiagramOutput struct {
	Status   string   `json:"status"`
	Artifact Artifact `json:"artifact"`
}

// ConceptCardInput is the input for the sketch_concept_card tool.
type ConceptCardInput struct {
	Title     string `json:"title" description:"Brief name for the concept"`
	Problem   string `json:"problem" description:"What user problem does this solve?"`
	Mechanism string `json:"mechanism" description:"How does it work?"`
	Value     string `json:"value" description:"What's the benefit?"`
	Risk      string `json:"risk" description:"What could go wrong?"`
}

// ConceptCardOutput is the output from the sketch_concept_card tool.
type ConceptCardOutput struct {
	Status string      `json:"status"`
	Card   ConceptCard `json:"card"`
}

// JourneyInput is the input for the sketch_journey tool.
type JourneyInput struct {
	Title  string            `json:"title" description:"Name for this journey"`
	Stages []JourneyStageIn  `json:"stages" description:"List of journey stages"`
}

// JourneyStageIn is a stage in the journey input.
type JourneyStageIn struct {
	Name       string `json:"name" description:"Stage name (e.g., Day 1, First Login)"`
	UserAction string `json:"user_action" description:"What the user does"`
	Emotion    string `json:"emotion" description:"How they feel (curious, confused, satisfied, etc.)"`
	Touchpoint string `json:"touchpoint" description:"Where this happens (app, email, etc.)"`
}

// JourneyOutput is the output from the sketch_journey tool.
type JourneyOutput struct {
	Status  string  `json:"status"`
	Journey Journey `json:"journey"`
}
