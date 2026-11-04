// Package main starts the TripGo HTTP service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/EgorTrytnev/TripGo/internal/config"
	"github.com/EgorTrytnev/TripGo/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		return
	}
	app, err := container.New(context.Background(), cfg)
	if err != nil {
		slog.Error("service initialization failed", "error", err)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("starting server", "address", cfg.HTTPAddr)
		serverErr <- app.Server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := app.Close(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = app.Close(shutdownCtx)
	}
}
