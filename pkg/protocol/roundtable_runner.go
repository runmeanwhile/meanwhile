package protocol

import (
	"context"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
)

func roundtableRunner(sess Session) roundtable.Runner {
	return func(ctx context.Context, ag agent.Agent, req roundtable.RunRequest) (agent.Message, error) {
		return sess.RunAgent(ctx, ag, RunRequest{
			Messages:          req.Messages,
			SystemMessages:    req.SystemMessages,
			Params:            req.Params,
			MaxToolIterations: req.MaxToolIterations,
			Tools:             req.Tools,
		})
	}
}
