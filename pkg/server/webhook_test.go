package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type webhookProtocol struct {
	participant protocol.Participant
}

func (p *webhookProtocol) ID() string { return "webhook.protocol" }
func (p *webhookProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.participant}
}
func (p *webhookProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *webhookProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.AwaitInput(ctx, p.participant, "need input", func(context.Context, agent.Message) error {
		return nil
	})
}
func (p *webhookProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *webhookProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestHumanResponseHandlerResponds(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	proto := &webhookProtocol{participant: human}
	sess, err := eng.Session("Webhook Test").
		Participant(human).
		Protocol(proto).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	err = sess.AwaitInput(context.Background(), human, "ctx", func(context.Context, agent.Message) error { return nil })
	var awaitErr *protocol.AwaitingInputError
	if err == nil || !errors.As(err, &awaitErr) {
		t.Fatalf("expected awaiting input error")
	}

	payload := HumanResponsePayload{
		RequestID: awaitErr.Request.RequestID,
		Response:  "answer",
		Source:    "webhook",
	}
	body, _ := json.Marshal(payload)

	handler := &HumanResponseHandler{Engine: eng}
	req := httptest.NewRequest(http.MethodPost, "/webhook/human-response", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(sess.PendingRequests()) != 0 {
		t.Fatalf("expected pending requests cleared")
	}
}

func TestHumanResponseHandlerVerifiesSignature(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	verifier, err := NewHMACVerifier("secret")
	if err != nil {
		t.Fatalf("NewHMACVerifier error = %v", err)
	}

	human := eng.Human("User").Build()
	proto := &webhookProtocol{participant: human}
	sess, err := eng.Session("Webhook Test").
		Participant(human).
		Protocol(proto).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	err = sess.AwaitInput(context.Background(), human, "ctx", func(context.Context, agent.Message) error { return nil })
	var awaitErr *protocol.AwaitingInputError
	if err == nil || !errors.As(err, &awaitErr) {
		t.Fatalf("expected awaiting input error")
	}

	payload := HumanResponsePayload{
		RequestID: awaitErr.Request.RequestID,
		Response:  "answer",
	}
	body, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	handler := &HumanResponseHandler{Engine: eng, Verifier: verifier}
	req := httptest.NewRequest(http.MethodPost, "/webhook/human-response", bytes.NewReader(body))
	req.Header.Set(signatureHeader, signature)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
