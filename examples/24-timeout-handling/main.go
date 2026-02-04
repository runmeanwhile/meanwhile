// Package main demonstrates scheduled timeout handling for human requests.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/scheduler"
)

type awaitProtocol struct {
	participant protocol.Participant
	responseCh  chan agent.Message
}

func (p *awaitProtocol) ID() string { return "timeout.demo" }
func (p *awaitProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.participant}
}
func (p *awaitProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *awaitProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.AwaitInput(ctx, p.participant, "Share a quick update (2s timeout).", func(_ context.Context, msg agent.Message) error {
		p.responseCh <- msg
		return nil
	}, protocol.WithInputTimeout(2*time.Second))
}
func (p *awaitProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *awaitProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng, err := engine.New(
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	driver := scheduler.NewInMemoryDriver()
	timeoutService, err := eng.NewTimeoutScheduler(driver, scheduler.WithInterval(100*time.Millisecond))
	if err != nil {
		log.Fatalf("failed to create timeout scheduler: %v", err)
	}
	if err := eng.SetTimeoutScheduler(timeoutService); err != nil {
		log.Fatalf("failed to set timeout scheduler: %v", err)
	}
	defer func() {
		_ = timeoutService.Close()
	}()

	go func() {
		if err := timeoutService.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("timeout scheduler stopped: %v", err)
		}
	}()

	human := eng.Human("User").Build()
	responseCh := make(chan agent.Message, 1)
	proto := &awaitProtocol{participant: human, responseCh: responseCh}

	sess, err := eng.Session("Timeout Demo").
		Participant(human).
		Protocol(proto).
		TimeoutPolicy(engine.ContinueWithNote("No response received; continuing.")).
		Build(ctx)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}

	result, err := sess.Run(ctx, message.User("start"))
	if err != nil {
		log.Fatalf("run error: %v", err)
	}
	if result.Status != engine.StatusAwaitingInput {
		log.Fatalf("expected awaiting input status")
	}

	fmt.Println("Waiting for timeout (2s)...")
	select {
	case msg := <-responseCh:
		fmt.Printf("Resolved with message: %s\n", msg.Text())
	case <-time.After(5 * time.Second):
		log.Fatal("timeout handler did not fire")
	}
}
