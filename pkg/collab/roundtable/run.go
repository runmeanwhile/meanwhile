package roundtable

import (
	"context"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// Turn represents a single agent turn request.
type Turn struct {
	Agent             agent.Agent
	Messages          []agent.Message
	SystemMessages    []agent.Message
	Params            map[string]any
	MaxToolIterations int
	Tools             []string
}

// TurnResult captures an agent response.
type TurnResult struct {
	Agent   agent.Agent
	Message agent.Message
}

// RunRequest mirrors the fields needed for agent runs.
type RunRequest struct {
	Messages          []agent.Message
	SystemMessages    []agent.Message
	Params            map[string]any
	MaxToolIterations int
	Tools             []string
}

// Runner executes a single turn request.
type Runner func(ctx context.Context, agent agent.Agent, req RunRequest) (agent.Message, error)

// RunSequential executes turns one by one.
func RunSequential(ctx context.Context, run Runner, turns []Turn) ([]TurnResult, error) {
	results := make([]TurnResult, 0, len(turns))
	for _, turn := range turns {
		resp, err := run(ctx, turn.Agent, RunRequest{
			Messages:          turn.Messages,
			SystemMessages:    turn.SystemMessages,
			Params:            turn.Params,
			MaxToolIterations: turn.MaxToolIterations,
			Tools:             turn.Tools,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, TurnResult{Agent: turn.Agent, Message: resp})
	}
	return results, nil
}

// ParallelConfig controls concurrency for parallel turns.
type ParallelConfig struct {
	MaxConcurrent int
}

// RunParallel executes turns concurrently with an optional concurrency cap.
func RunParallel(ctx context.Context, run Runner, turns []Turn, cfg ParallelConfig) ([]TurnResult, error) {
	results := make([]TurnResult, len(turns))
	var wg sync.WaitGroup
	errCh := make(chan error, len(turns))
	var sem chan struct{}
	if cfg.MaxConcurrent > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrent)
	}

	for i, turn := range turns {
		i := i
		turn := turn
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			resp, err := run(ctx, turn.Agent, RunRequest{
				Messages:          turn.Messages,
				SystemMessages:    turn.SystemMessages,
				Params:            turn.Params,
				MaxToolIterations: turn.MaxToolIterations,
				Tools:             turn.Tools,
			})
			if err != nil {
				errCh <- err
				return
			}
			results[i] = TurnResult{Agent: turn.Agent, Message: resp}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
