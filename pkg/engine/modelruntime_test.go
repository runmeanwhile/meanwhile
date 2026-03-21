package engine

import (
	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime/compat"
)

func runtimeTextMessage(role modelruntime.Role, text string) modelruntime.Message {
	return modelruntime.Message{
		Role:  role,
		Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: text}},
	}
}

func runtimeFromAgentMessage(msg agent.Message) modelruntime.Message {
	return compat.FromAgentMessage(msg)
}
