package requestregistry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/redis/go-redis/v9"
)

const defaultRedisKeyPrefix = "meanwhile:request-registry"

// RedisOption configures the Redis-backed request registry.
type RedisOption func(*redisConfig)

type redisConfig struct {
	client    redis.UniversalClient
	keyPrefix string
	ttl       time.Duration
}

// WithRedisClient provides a Redis client instance.
func WithRedisClient(client redis.UniversalClient) RedisOption {
	return func(cfg *redisConfig) {
		cfg.client = client
	}
}

// WithKeyPrefix overrides the key prefix used for request IDs.
func WithKeyPrefix(prefix string) RedisOption {
	return func(cfg *redisConfig) {
		cfg.keyPrefix = prefix
	}
}

// WithTTL sets a default TTL for request entries.
func WithTTL(ttl time.Duration) RedisOption {
	return func(cfg *redisConfig) {
		if ttl > 0 {
			cfg.ttl = ttl
		}
	}
}

// RedisRegistry stores request mappings in Redis.
type RedisRegistry struct {
	client    redis.UniversalClient
	keyPrefix string
	ttl       time.Duration
}

// NewRedisRegistry creates a Redis-backed request registry.
func NewRedisRegistry(options redis.UniversalOptions, opts ...RedisOption) (*RedisRegistry, error) {
	cfg := redisConfig{
		keyPrefix: defaultRedisKeyPrefix,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.client == nil {
		if len(options.Addrs) == 0 && options.MasterName == "" {
			return nil, fmt.Errorf("redis options required")
		}
		cfg.client = redis.NewUniversalClient(&options)
	}
	prefix := strings.TrimSpace(cfg.keyPrefix)
	if prefix == "" {
		prefix = defaultRedisKeyPrefix
	}
	return &RedisRegistry{
		client:    cfg.client,
		keyPrefix: prefix,
		ttl:       cfg.ttl,
	}, nil
}

// Register records a request -> session mapping.
func (r *RedisRegistry) Register(ctx context.Context, requestID, sessionID string) error {
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("request id required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := r.key(requestID)
	var ok bool
	var err error
	if r.ttl > 0 {
		ok, err = r.client.SetNX(ctx, key, sessionID, r.ttl).Result()
	} else {
		ok, err = r.client.SetNX(ctx, key, sessionID, 0).Result()
	}
	if err != nil {
		return fmt.Errorf("register request: %w", err)
	}
	if ok {
		return nil
	}
	existing, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("register request: %w", err)
	}
	if existing == sessionID {
		if r.ttl > 0 {
			_ = r.client.Expire(ctx, key, r.ttl)
		}
		return nil
	}
	return fmt.Errorf("request already registered")
}

// Resolve returns the session ID for a request.
func (r *RedisRegistry) Resolve(ctx context.Context, requestID string) (string, error) {
	if strings.TrimSpace(requestID) == "" {
		return "", fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	value, err := r.client.Get(ctx, r.key(requestID)).Result()
	if err == redis.Nil {
		return "", engine.ErrRequestNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve request: %w", err)
	}
	return value, nil
}

// Delete removes a request mapping.
func (r *RedisRegistry) Delete(ctx context.Context, requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	removed, err := r.client.Del(ctx, r.key(requestID)).Result()
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	if removed == 0 {
		return engine.ErrRequestNotFound
	}
	return nil
}

// Close closes the underlying Redis client.
func (r *RedisRegistry) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *RedisRegistry) key(requestID string) string {
	return r.keyPrefix + ":" + requestID
}
