// Package message provides message constructors and types for agent communication.
//
// Messages are the primary means of communication in Meanwhile sessions.
// Use constructors to create messages with the appropriate role:
//
//	msg := message.User("What's the issue?")
//	response := message.Assistant("The printer is out of paper.")
//	context := message.System("You are a helpful assistant.")
//
// Structured content parts (text + images) are supported:
//
//	msg := message.UserParts(
//	    message.TextPart("What does this diagram show?"),
//	    message.ImagePart("https://example.com/diagram.png"),
//	)
//
// Meanwhile... messages flow between agents, protocols orchestrate, and work gets done.
package message
