package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/EgorTrytnev/TripGo/internal/config"
	"github.com/EgorTrytnev/TripGo/internal/db"
	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/handler"
	"github.com/EgorTrytnev/TripGo/internal/repository"
	"github.com/EgorTrytnev/TripGo/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration failed: %v", err)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout+cfg.DatabaseQueryTimeout)
	defer cancelStartup()
	pool, err := db.NewPool(startupCtx, cfg)
	if err != nil {
		log.Fatalf("database startup failed: %v", err)
	}
	defer pool.Close()

	txManager := db.NewTxManager(pool, cfg.DatabaseQueryTimeout)
	tripRepo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	tripService := service.NewTripService(tripRepo, txManager)
	apiHandler := handler.NewHandler(tripService, pool, cfg.DatabaseQueryTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	api.HandlerWithOptions(apiHandler, api.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: apiHandler.OpenAPIErrorHandler,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("starting server on %s", cfg.HTTPAddr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v; closing server", err)
			if closeErr := srv.Close(); closeErr != nil {
				log.Printf("force close failed: %v", closeErr)
			}
		} else {
			log.Println("server stopped")
		}
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}
}
