// Package main starts the TripGo HTTP service.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/EgorTrytnev/TripGo/internal/config"
	"github.com/EgorTrytnev/TripGo/internal/container"
	"github.com/EgorTrytnev/TripGo/internal/infra/telemetry"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := telemetry.NewLogger("info")
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		return 1
	}
	logger = telemetry.NewLogger(cfg.LogLevel)
	app, err := container.New(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("service initialization failed", "error", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "address", cfg.HTTPAddr)
		serverErr <- app.Server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		if err := app.Close(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		cancel()
	case err := <-serverErr:
		failed := err != nil && !errors.Is(err, http.ErrServerClosed)
		if failed {
			logger.Error("server stopped unexpectedly", "error", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		_ = app.Close(shutdownCtx)
		cancel()
		if failed {
			return 1
		}
	}
	return 0
}
