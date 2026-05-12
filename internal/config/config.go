// Package config loads runtime configuration from environment variables.
// All defaults live here so main.go reads like a recipe.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the validated configuration the rest of the app consumes.
// It is populated once at startup and treated as immutable.
type Config struct {
	Port                     string
	DatabaseURL              string
	RedisURL                 string
	LogLevel                 string
	OtelEndpoint             string
	CacheTTL                 time.Duration
	MaxCandidatesPerQuery    int
	ShutdownGracePeriod      time.Duration
}

// Load reads env vars and returns a populated Config. Missing required
// values produce a descriptive error rather than a runtime panic later.
func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		LogLevel:              getenv("LOG_LEVEL", "info"),
		OtelEndpoint:          os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		MaxCandidatesPerQuery: getenvInt("MAX_CANDIDATES_PER_QUERY", 100),
		ShutdownGracePeriod:   getenvDuration("SHUTDOWN_GRACE", 10*time.Second),
	}

	ttl, err := time.ParseDuration(getenv("CACHE_TTL", "5m"))
	if err != nil {
		return Config{}, fmt.Errorf("config: CACHE_TTL: %w", err)
	}
	cfg.CacheTTL = ttl

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
