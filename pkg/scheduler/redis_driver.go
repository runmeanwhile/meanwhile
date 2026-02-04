package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisKeyPrefix = "meanwhile:scheduler"

// RedisOption configures the Redis driver.
type RedisOption func(*redisDriverConfig)

type redisDriverConfig struct {
	client    redis.UniversalClient
	keyPrefix string
}

// WithRedisClient provides a Redis client instance.
func WithRedisClient(client redis.UniversalClient) RedisOption {
	return func(cfg *redisDriverConfig) {
		cfg.client = client
	}
}

// WithRedisKeyPrefix overrides the key prefix used for Redis keys.
func WithRedisKeyPrefix(prefix string) RedisOption {
	return func(cfg *redisDriverConfig) {
		cfg.keyPrefix = prefix
	}
}

// RedisDriver stores scheduled jobs in Redis.
type RedisDriver struct {
	client      redis.UniversalClient
	keyPrefix   string
	claimScript *redis.Script
}

// NewRedisDriver creates a Redis-backed driver using the provided options.
func NewRedisDriver(options redis.UniversalOptions, opts ...RedisOption) (*RedisDriver, error) {
	cfg := redisDriverConfig{
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
		client := redis.NewUniversalClient(&options)
		cfg.client = client
	}
	prefix := strings.TrimSpace(cfg.keyPrefix)
	if prefix == "" {
		prefix = defaultRedisKeyPrefix
	}
	return &RedisDriver{
		client:    cfg.client,
		keyPrefix: prefix,
		claimScript: redis.NewScript(`
local schedule_key = KEYS[1]
local job_key = KEYS[2]
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])

local ids = redis.call("ZRANGEBYSCORE", schedule_key, "-inf", now, "LIMIT", 0, limit)
if #ids == 0 then
  return {}
end

redis.call("ZREM", schedule_key, unpack(ids))
local payloads = redis.call("HMGET", job_key, unpack(ids))
for _, id in ipairs(ids) do
  redis.call("HDEL", job_key, id)
end

local out = {}
for i, id in ipairs(ids) do
  table.insert(out, id)
  table.insert(out, payloads[i])
end
return out
`),
	}, nil
}

// Schedule upserts a job.
func (d *RedisDriver) Schedule(ctx context.Context, job Job) error {
	if err := validateJob(job); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	job.RunAt = job.RunAt.UTC()
	raw, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	score := float64(job.RunAt.UnixMilli())
	pipe := d.client.TxPipeline()
	pipe.HSet(ctx, d.jobKey(), job.ID, raw)
	pipe.ZAdd(ctx, d.scheduleKey(), redis.Z{Score: score, Member: job.ID})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("schedule job: %w", err)
	}
	return nil
}

// Cancel removes a job.
func (d *RedisDriver) Cancel(ctx context.Context, jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return fmt.Errorf("job id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pipe := d.client.TxPipeline()
	hdel := pipe.HDel(ctx, d.jobKey(), jobID)
	zrem := pipe.ZRem(ctx, d.scheduleKey(), jobID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cancel job: %w", err)
	}
	if hdel.Val() == 0 && zrem.Val() == 0 {
		return ErrJobNotFound
	}
	return nil
}

// ClaimDue claims jobs due on or before now.
func (d *RedisDriver) ClaimDue(ctx context.Context, now time.Time, limit int) ([]Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	res, err := d.claimScript.Run(ctx, d.client, []string{d.scheduleKey(), d.jobKey()}, now.UnixMilli(), limit).Result()
	if err != nil {
		return nil, fmt.Errorf("claim due jobs: %w", err)
	}
	values, ok := res.([]any)
	if !ok || len(values) == 0 {
		return nil, nil
	}
	jobs := make([]Job, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		jobID := toString(values[i])
		raw := toString(values[i+1])
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var job Job
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			return nil, fmt.Errorf("decode job: %w", err)
		}
		if job.ID == "" {
			job.ID = jobID
		} else if jobID != "" && job.ID != jobID {
			return nil, fmt.Errorf("job id mismatch")
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// Close closes the underlying Redis client.
func (d *RedisDriver) Close() error {
	if d == nil || d.client == nil {
		return nil
	}
	return d.client.Close()
}

func (d *RedisDriver) scheduleKey() string {
	return d.keyPrefix + ":schedule"
}

func (d *RedisDriver) jobKey() string {
	return d.keyPrefix + ":jobs"
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
