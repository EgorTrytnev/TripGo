package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Providers groups the initialized logging, tracing and metric dependencies.
type Providers struct {
	Tracer  *sdktrace.TracerProvider
	Meter   *metric.MeterProvider
	Metrics *BusinessMetrics
	Logger  *slog.Logger
}

// Init configures slog and OTLP exporters from environment variables.
func Init(ctx context.Context, level string) (*Providers, error) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	logger := slog.New(&traceHandler{next: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})})
	logger = logger.With("service", "trip-service")
	protocol := strings.TrimSpace(strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	if protocol == "" {
		protocol = "grpc"
	}
	var traceExporter sdktrace.SpanExporter
	var metricExporter metric.Exporter
	var err error
	switch protocol {
	case "grpc":
		traceExporter, err = otlptracegrpc.New(ctx)
		if err == nil {
			metricExporter, err = otlpmetricgrpc.New(ctx)
		}
	case "http/protobuf", "http":
		traceExporter, err = otlptracehttp.New(ctx)
		if err == nil {
			metricExporter, err = otlpmetrichttp.New(ctx)
		}
	default:
		return nil, fmt.Errorf("unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q", protocol)
	}
	if err != nil {
		return nil, err
	}
	res := resource.NewWithAttributes("", attribute.String("service.name", "trip-service"))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res), sdktrace.WithSampler(samplerFromEnv()))
	mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)), metric.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	bm, err := NewBusinessMetrics(mp.Meter("tripgo"))
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil, err
	}
	return &Providers{Tracer: tp, Meter: mp, Metrics: bm, Logger: logger}, nil
}

func samplerFromEnv() sdktrace.Sampler {
	s := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	switch s {
	case "always_off":
		return sdktrace.NeverSample()
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "always_on", "":
		return sdktrace.AlwaysSample()
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "traceidratio", "parentbased_traceidratio":
		ratio, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")), 64)
		if err != nil {
			ratio = 0.1
		}
		sampler := sdktrace.TraceIDRatioBased(ratio)
		if s == "parentbased_traceidratio" {
			return sdktrace.ParentBased(sampler)
		}
		return sampler
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

type traceHandler struct{ next slog.Handler }

func (h *traceHandler) Enabled(ctx context.Context, l slog.Level) bool { return h.next.Enabled(ctx, l) }

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.next.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &traceHandler{next: h.next.WithAttrs(a)}
}

func (h *traceHandler) WithGroup(n string) slog.Handler {
	return &traceHandler{next: h.next.WithGroup(n)}
}
