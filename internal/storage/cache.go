package storage

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var cacheTracer = otel.Tracer("fan-selector-api/cache")

// Cache is the small surface the API needs from any caching layer.
// Both the real Redis client and the NopCache satisfy it.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}

// NopCache satisfies Cache but never stores anything. It exists so the rest
// of the system can be wired up identically with or without Redis configured.
type NopCache struct{}

func (NopCache) Get(context.Context, string) ([]byte, bool, error)              { return nil, false, nil }
func (NopCache) Set(context.Context, string, []byte, time.Duration) error       { return nil }
func (NopCache) Ping(context.Context) error                                     { return nil }
func (NopCache) Close() error                                                   { return nil }

// RedisCache wraps a go-redis client and adds TTL jitter.
type RedisCache struct {
	client *redis.Client
	jitter float64 // ±fraction, e.g. 0.10 for ±10%
}

// NewRedis returns a RedisCache when dsn is non-empty, otherwise NopCache.
// An invalid dsn returns an error so misconfiguration is caught at startup
// instead of silently disabling caching.
func NewRedis(ctx context.Context, dsn string) (Cache, error) {
	if dsn == "" {
		return NopCache{}, nil
	}
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("cache: parse dsn: %w", err)
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: ping: %w", err)
	}
	return &RedisCache{client: client, jitter: 0.10}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, span := cacheTracer.Start(ctx, "cache.get")
	defer span.End()

	val, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache: get: %w", err)
	}
	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int("cache.value_bytes", len(val)),
	)
	return val, true, nil
}

// Set applies jitter to the TTL so cache entries inserted in the same burst
// don't all expire at the same instant.
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, span := cacheTracer.Start(ctx, "cache.set")
	defer span.End()

	jittered := r.applyJitter(ttl)
	span.SetAttributes(
		attribute.Int("cache.value_bytes", len(value)),
		attribute.Int64("cache.ttl_ms", jittered.Milliseconds()),
	)
	if err := r.client.Set(ctx, key, value, jittered).Err(); err != nil {
		return fmt.Errorf("cache: set: %w", err)
	}
	return nil
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) applyJitter(ttl time.Duration) time.Duration {
	if r.jitter <= 0 {
		return ttl
	}
	// Random multiplier in [1-jitter, 1+jitter].
	mul := 1 + (rand.Float64()*2-1)*r.jitter
	return time.Duration(float64(ttl) * mul)
}
