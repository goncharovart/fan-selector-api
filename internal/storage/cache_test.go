package storage

import (
	"context"
	"testing"
	"time"
)

func TestNopCache_AlwaysMissesAndNeverErrors(t *testing.T) {
	c := NopCache{}
	ctx := context.Background()

	if err := c.Ping(ctx); err != nil {
		t.Errorf("Ping should not error: %v", err)
	}
	if _, hit, err := c.Get(ctx, "anything"); err != nil || hit {
		t.Errorf("expected miss, got hit=%v err=%v", hit, err)
	}
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Errorf("Set should be a no-op: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close should be a no-op: %v", err)
	}
}

func TestRedisCache_JitterStaysInBounds(t *testing.T) {
	r := &RedisCache{jitter: 0.10}
	base := 5 * time.Minute
	for i := 0; i < 100; i++ {
		got := r.applyJitter(base)
		lo := time.Duration(float64(base) * 0.9)
		hi := time.Duration(float64(base) * 1.1)
		if got < lo || got > hi {
			t.Errorf("jittered TTL %v outside [%v, %v]", got, lo, hi)
		}
	}
}

func TestRedisCache_ZeroJitterIsIdentity(t *testing.T) {
	r := &RedisCache{jitter: 0}
	base := 5 * time.Minute
	if got := r.applyJitter(base); got != base {
		t.Errorf("expected %v, got %v", base, got)
	}
}
