// Package container constructs and connects the service dependencies.
package container

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/EgorTrytnev/TripGo/internal/config"
	api "github.com/EgorTrytnev/TripGo/internal/generated"
	httpmiddleware "github.com/EgorTrytnev/TripGo/internal/http/middleware"
	httptrip "github.com/EgorTrytnev/TripGo/internal/http/trip"
	"github.com/EgorTrytnev/TripGo/internal/infra/postgres"
	"github.com/EgorTrytnev/TripGo/internal/infra/telemetry"
	"github.com/EgorTrytnev/TripGo/internal/repository"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/finish"
	gettrip "github.com/EgorTrytnev/TripGo/internal/usecase/trip/get"
)

// App holds the service server and resources that must be closed on shutdown.
type App struct {
	Server            *http.Server
	poolCloser        interface{ Close() }
	shutdownTelemetry func(context.Context) error
}

// New builds the service dependencies and HTTP server.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	providers, err := telemetry.Init(ctx, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}
	startupCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout+cfg.DatabaseQueryTimeout)
	defer cancel()
	dbTracer, err := telemetry.NewPGXTracer(providers.Meter.Meter("tripgo.postgres"), providers.Tracer)
	if err != nil {
		_ = providers.Tracer.Shutdown(ctx)
		_ = providers.Meter.Shutdown(ctx)
		return nil, fmt.Errorf("initialize PostgreSQL telemetry: %w", err)
	}
	pool, err := postgres.NewPool(startupCtx, cfg, dbTracer)
	if err != nil {
		_ = providers.Tracer.Shutdown(ctx)
		_ = providers.Meter.Shutdown(ctx)
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	tx := postgres.NewTxManager(pool, cfg.DatabaseQueryTimeout)
	repo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	createUC := create.New(repo, tx, providers.Metrics, providers.Logger)
	getUC := gettrip.New(repo)
	finishUC := finish.New(repo, tx, providers.Metrics, providers.Logger)
	handler := httptrip.NewHandler(createUC, getUC, finishUC, pool, cfg.DatabaseQueryTimeout, providers.Logger)
	r := chi.NewRouter()
	r.Use(httpmiddleware.CaptureRoute)
	httpMetrics, err := httpmiddleware.NewHTTPMetrics(providers.Meter.Meter("tripgo.http"))
	if err != nil {
		pool.Close()
		_ = providers.Tracer.Shutdown(ctx)
		_ = providers.Meter.Shutdown(ctx)
		return nil, err
	}
	api.HandlerWithOptions(handler, api.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: handler.OpenAPIErrorHandler})
	root := httpmiddleware.Chain(r, providers.Logger, httpMetrics)
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: root, ReadTimeout: cfg.ReadTimeout, ReadHeaderTimeout: cfg.ReadHeaderTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout}
	return &App{Server: srv, poolCloser: pool, shutdownTelemetry: func(c context.Context) error {
		err1 := providers.Tracer.Shutdown(c)
		err2 := providers.Meter.Shutdown(c)
		if err1 != nil {
			return err1
		}
		return err2
	}}, nil
}

// Close gracefully stops the server and closes its infrastructure resources.
func (a *App) Close(ctx context.Context) error {
	var result error
	if err := a.Server.Shutdown(ctx); err != nil {
		result = err
		_ = a.Server.Close()
	}
	if a.poolCloser != nil {
		a.poolCloser.Close()
	}
	if a.shutdownTelemetry != nil {
		if err := a.shutdownTelemetry(ctx); err != nil && result == nil {
			result = err
		}
	}
	return result
}
