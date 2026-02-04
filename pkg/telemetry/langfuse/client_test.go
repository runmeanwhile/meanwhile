package langfuse

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/telemetry"
)

func TestLangfuseClientSendsTraces(t *testing.T) {
	var reqCount int32
	var authHeader string
	var reqPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		authHeader = r.Header.Get("Authorization")
		reqPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		PublicKey: "pk_test",
		SecretKey: "sk_test",
		Endpoint:  server.URL + "/api/public/otel",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	traceSpan, ctx := client.StartTrace(context.Background(), telemetry.SpanInput{Name: "session.run"})
	child, _ := client.StartSpan(ctx, telemetry.SpanInput{Name: "agent.run"})
	child.End(nil)
	traceSpan.End(nil)

	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}

	if atomic.LoadInt32(&reqCount) == 0 {
		t.Fatal("expected OTLP request")
	}

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk_test:sk_test"))
	if authHeader != expectedAuth {
		t.Fatalf("unexpected auth header: %s", authHeader)
	}

	if reqPath != "/api/public/otel/v1/traces" {
		t.Fatalf("unexpected path: %s", reqPath)
	}
}
