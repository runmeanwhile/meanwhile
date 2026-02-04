// Package main runs a webhook server for human responses.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/server"
)

type awaitProtocol struct {
	participant protocol.Participant
}

func (p *awaitProtocol) ID() string { return "webhook.await" }
func (p *awaitProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.participant}
}
func (p *awaitProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *awaitProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.AwaitInput(ctx, p.participant, "Please respond via webhook.", func(context.Context, agent.Message) error {
		return nil
	})
}
func (p *awaitProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *awaitProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func main() {
	ctx := context.Background()

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("failed to create provider: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel("gpt-5-mini"),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	human := eng.Human("User").Build()
	proto := &awaitProtocol{participant: human}
	sess, err := eng.Session("Webhook Receiver").
		Participant(human).
		Protocol(proto).
		Build(ctx)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}

	result, err := sess.Run(ctx, message.User("start"))
	if err != nil {
		log.Fatalf("run error: %v", err)
	}
	if result.Status != engine.StatusAwaitingInput || result.RequestID == "" {
		log.Fatalf("expected awaiting input status")
	}

	fmt.Printf("Request ID: %s\n", result.RequestID)
	fmt.Println("Send a POST to http://localhost:8080/webhook/human-response with JSON:")
	fmt.Printf("{\"request_id\":\"%s\",\"response\":\"your answer\"}\n", result.RequestID)

	handler := &server.HumanResponseHandler{Engine: eng}
	http.Handle("/webhook/human-response", handler)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
