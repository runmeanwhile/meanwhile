package requestregistry

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/redis/go-redis/v9"
)

func TestRedisRegistryRegisterResolveDelete(t *testing.T) {
	redisServer := miniredis.RunT(t)
	registry, err := NewRedisRegistry(redis.UniversalOptions{Addrs: []string{redisServer.Addr()}})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	if err := registry.Register(context.Background(), "req-1", "sess-1"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := registry.Register(context.Background(), "req-1", "sess-1"); err != nil {
		t.Fatalf("register idempotent: %v", err)
	}
	if err := registry.Register(context.Background(), "req-1", "sess-2"); err == nil {
		t.Fatalf("expected error on different session")
	}

	sessionID, err := registry.Resolve(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if sessionID != "sess-1" {
		t.Fatalf("expected sess-1, got %s", sessionID)
	}

	if err := registry.Delete(context.Background(), "req-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := registry.Delete(context.Background(), "req-1"); err != engine.ErrRequestNotFound {
		t.Fatalf("expected ErrRequestNotFound, got %v", err)
	}
}

func TestRedisRegistryTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	registry, err := NewRedisRegistry(
		redis.UniversalOptions{Addrs: []string{redisServer.Addr()}},
		WithTTL(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	if err := registry.Register(context.Background(), "req-ttl", "sess-ttl"); err != nil {
		t.Fatalf("register: %v", err)
	}
	redisServer.FastForward(200 * time.Millisecond)

	if _, err := registry.Resolve(context.Background(), "req-ttl"); err != engine.ErrRequestNotFound {
		t.Fatalf("expected ErrRequestNotFound, got %v", err)
	}
}
