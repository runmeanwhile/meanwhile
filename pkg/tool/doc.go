// Package tool provides typed tools for agent actions.
//
// # Creating Tools
//
// Create tools from typed functions with automatic schema generation:
//
//	type TicketArgs struct {
//	    Issue    string `json:"issue" description:"The issue to report"`
//	    Priority string `json:"priority" description:"Priority: low, medium, high"`
//	}
//
//	ticketTool, err := tool.New("create_ticket", func(ctx context.Context, args TicketArgs) (string, error) {
//	    return fmt.Sprintf("Ticket created: %s", args.Issue), nil
//	})
//
// Add a description with fluent chaining:
//
//	ticketTool, err := tool.New("create_ticket", handler).
//	    WithDescription("Create a support ticket for an issue")
//
// # Using Tools with Agents
//
// Pass tools directly to agents - they're automatically registered:
//
//	agent := eng.Agent("Support").
//	    Tools(ticketTool, restartTool).  // Registers + adds
//	    Build()
//
// # Schema Generation
//
// Tool schemas are derived from your struct fields:
//
//   - `json` tag: field name in schema
//   - `description` tag: field description
//   - Struct field tags become JSON schema properties
//
// The LLM receives the schema and calls your tool with validated arguments.
//
// Meanwhile... tools just work.
package tool
