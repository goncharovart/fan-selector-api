// Command server is the entrypoint for fan-selector-api.
// It wires config, observability, storage, and HTTP handlers, then runs
// until a SIGTERM arrives.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goncharovart/fan-selector-api/internal/api"
	"github.com/goncharovart/fan-selector-api/internal/config"
	"github.com/goncharovart/fan-selector-api/internal/observability"
	"github.com/goncharovart/fan-selector-api/internal/storage"
)

func main() {
	if err := run(); err != nil {
		// Use plain stderr here — the logger may not be initialized yet.
		_, _ = os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg.LogLevel)
	logger.Info("starting", "version", buildVersion(), "port", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	shutdownTracer, err := observability.SetupTracer(ctx, cfg.OtelEndpoint, "fan-selector-api")
	if err != nil {
		// Tracing is optional; continue without it but warn loudly.
		logger.Warn("tracer setup failed, continuing without traces", "err", err)
		shutdownTracer = func(context.Context) error { return nil }
	}

	store, err := storage.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	cache, err := storage.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		// Hard fail when REDIS_URL is set but unreachable — silent fallback
		// would surprise the operator. Leave REDIS_URL empty to disable.
		return err
	}
	defer cache.Close()

	handler := api.NewHandler(store, store, cache, logger, cfg.CacheTTL, cfg.MaxCandidatesPerQuery)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()
	logger.Info("listening", "addr", server.Addr)

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGracePeriod)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown error", "err", err)
	}
	if err := shutdownTracer(shutdownCtx); err != nil {
		logger.Warn("tracer shutdown error", "err", err)
	}
	logger.Info("bye")
	return nil
}

// buildVersion is overridden via -ldflags at build time; useful in logs.
var version = "dev"

func buildVersion() string { return version }
