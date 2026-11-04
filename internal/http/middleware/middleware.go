// Package middleware provides HTTP recovery, tracing, logging and metrics.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMetrics contains the request instruments for the HTTP server.
type HTTPMetrics struct {
	Duration metric.Float64Histogram
	Active   metric.Int64UpDownCounter
}

// NewHTTPMetrics creates the required HTTP duration and active-request instruments.
func NewHTTPMetrics(m metric.Meter) (*HTTPMetrics, error) {
	d, err := m.Float64Histogram("http.server.request.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	a, err := m.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, err
	}
	return &HTTPMetrics{Duration: d, Active: a}, nil
}

// Chain applies Recover, Tracing, Logging and Metrics around the router.
func Chain(next http.Handler, logger *slog.Logger, metrics *HTTPMetrics) http.Handler {
	wrapped := Recover(logger, Tracing(Logging(logger, Metrics(metrics, next))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		holder := &routeHolder{}
		wrapped.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeHolderKey{}, holder)))
	})
}

type (
	routeHolderKey struct{}
	routeHolder    struct{ pattern string }
)

// CaptureRoute attaches the matched chi route template to request telemetry.
func CaptureRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
					span.SetStatus(codes.Error, "HTTP handler panicked")
					span.SetAttributes(attribute.String("error.type", "panic"))
				}
				panic(recovered)
			}
		}()
		next.ServeHTTP(wrapped, r)
		if holder, ok := r.Context().Value(routeHolderKey{}).(*routeHolder); ok {
			if rc := chi.RouteContext(r.Context()); rc != nil {
				holder.pattern = rc.RoutePattern()
			}
		}
		if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
			route := routePattern(r)
			span.SetName(r.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route), attribute.Int("http.response.status_code", wrapped.Status()))
			if wrapped.Status() >= 500 {
				span.SetStatus(codes.Error, "HTTP server error")
				span.SetAttributes(attribute.String("error.type", "http.server_error"))
			}
		}
	})
}

// Recover converts handler panics into internal_error responses.
func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusWriter{ResponseWriter: w}
		defer func() {
			if v := recover(); v != nil {
				logger.ErrorContext(r.Context(), "request panicked", "panic", v, "stack", string(debug.Stack()))
				if wrapped.status == 0 {
					problem(w, http.StatusInternalServerError, "internal_error")
				}
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

// Tracing creates an OpenTelemetry server span for each request.
func Tracing(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "trip-service", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " " + routeFromRequest(r) }))
}

// Logging records one structured event after a request completes.
func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		defer func() {
			recovered := recover()
			status := wrapped.Status()
			if recovered != nil {
				status = http.StatusInternalServerError
			}
			attrs := []any{"method", r.Method, "route", routePattern(r), "status", status, "duration_ms", time.Since(start).Milliseconds()}
			if status >= 500 {
				logger.ErrorContext(r.Context(), "request handled", attrs...)
			} else {
				logger.InfoContext(r.Context(), "request handled", attrs...)
			}
			if recovered != nil {
				panic(recovered)
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

// Metrics records request duration and active request count.
func Metrics(m *HTTPMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method
		initial := []attribute.KeyValue{attribute.String("http.request.method", method), attribute.String("http.route", routeFromRequest(r))}
		if m != nil {
			m.Active.Add(r.Context(), 1, metric.WithAttributes(initial...))
			defer m.Active.Add(r.Context(), -1, metric.WithAttributes(initial...))
		}
		wrapped := &statusWriter{ResponseWriter: w}
		defer func() {
			recovered := recover()
			if m != nil {
				status := wrapped.Status()
				if recovered != nil {
					status = http.StatusInternalServerError
				}
				attrs := []attribute.KeyValue{initial[0], attribute.String("http.route", routePattern(r)), attribute.Int("http.response.status_code", status)}
				m.Duration.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(attrs...))
			}
			if recovered != nil {
				panic(recovered)
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

func routeFromRequest(r *http.Request) string {
	path := r.URL.Path
	switch {
	case path == "/health" && r.Method == http.MethodGet:
		return "/health"
	case path == "/ready" && r.Method == http.MethodGet:
		return "/ready"
	case path == "/api/v1/trips" && r.Method == http.MethodPost:
		return "/api/v1/trips"
	case strings.HasPrefix(path, "/api/v1/trips/") && strings.HasSuffix(path, "/finish") && r.Method == http.MethodPost:
		return "/api/v1/trips/{tripId}/finish"
	case strings.HasPrefix(path, "/api/v1/trips/") && r.Method == http.MethodGet:
		return "/api/v1/trips/{tripId}"
	default:
		return "unmatched"
	}
}

func routePattern(r *http.Request) string {
	if holder, ok := r.Context().Value(routeHolderKey{}).(*routeHolder); ok && holder.pattern != "" {
		return holder.pattern
	}
	if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
		return rc.RoutePattern()
	}
	return "unmatched"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func problem(w http.ResponseWriter, status int, code string) {
	detail := "An unexpected error occurred"
	body := map[string]any{"type": "https://tripgo.example/problems/" + code, "title": "Internal Server Error", "status": status, "detail": detail, "code": code}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
