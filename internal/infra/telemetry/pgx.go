package telemetry

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// PGXTracer adds OpenTelemetry query spans and duration metrics to pgx.
type PGXTracer struct {
	base     *otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryState struct {
	start                 time.Time
	operation, collection string
}

type queryStateKey struct{}

// NewPGXTracer creates a pgx tracer using the supplied providers.
func NewPGXTracer(m metric.Meter, tp trace.TracerProvider) (*PGXTracer, error) {
	duration, err := m.Float64Histogram("db.client.operation.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	base := otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tp),
		otelpgx.WithMeterProvider(metricnoop.NewMeterProvider()),
		otelpgx.WithDisableSQLStatementInAttributes(),
		otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
	)
	return &PGXTracer{base: base, duration: duration}, nil
}

// TraceQueryStart starts tracing and timing a pgx query.
func (t *PGXTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.base.TraceQueryStart(ctx, conn, data)
	operation, collection := queryInfo(data.SQL)
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("db.operation.name", operation), attribute.String("db.collection.name", collection))
	return context.WithValue(ctx, queryStateKey{}, queryState{start: time.Now(), operation: operation, collection: collection})
}

// TraceQueryEnd records query duration and ends its span.
func (t *PGXTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", reflect.TypeOf(data.Err).String()))
	}
	t.base.TraceQueryEnd(ctx, conn, data)
	state, ok := ctx.Value(queryStateKey{}).(queryState)
	if !ok {
		return
	}
	attrs := []attribute.KeyValue{attribute.String("db.operation.name", state.operation), attribute.String("db.collection.name", state.collection)}
	if data.Err != nil {
		attrs = append(attrs, attribute.String("error.type", reflect.TypeOf(data.Err).String()))
	}
	t.duration.Record(ctx, time.Since(state.start).Seconds(), metric.WithAttributes(attrs...))
}

func queryInfo(sql string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(sql))
	if len(fields) == 0 {
		return "UNKNOWN", "unknown"
	}
	operation, table := strings.ToUpper(fields[0]), "unknown"
	switch operation {
	case "INSERT":
		for i := 1; i+1 < len(fields); i++ {
			if strings.EqualFold(fields[i], "into") {
				table = cleanTable(fields[i+1])
				break
			}
		}
	case "SELECT", "DELETE":
		for i := 1; i+1 < len(fields); i++ {
			if strings.EqualFold(fields[i], "from") {
				table = cleanTable(fields[i+1])
				break
			}
		}
	case "UPDATE":
		if len(fields) > 1 {
			table = cleanTable(fields[1])
		}
	}
	return operation, table
}

func cleanTable(name string) string { return strings.Trim(name, "\"`; ") }
