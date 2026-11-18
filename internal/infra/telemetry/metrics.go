// Package telemetry initializes and instruments OpenTelemetry signals.
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// BusinessMetrics contains counters for trip lifecycle events.
type BusinessMetrics struct {
	Created   metric.Int64Counter
	Completed metric.Int64Counter
}

// NewBusinessMetrics creates trip lifecycle counters.
func NewBusinessMetrics(m metric.Meter) (*BusinessMetrics, error) {
	created, err := m.Int64Counter("tripgo.trips.created", metric.WithDescription("Number of created trips"))
	if err != nil {
		return nil, err
	}
	completed, err := m.Int64Counter("tripgo.trips.completed", metric.WithDescription("Number of completed trips"))
	if err != nil {
		return nil, err
	}
	return &BusinessMetrics{Created: created, Completed: completed}, nil
}

// AddCreated records newly persisted trips.
func (m *BusinessMetrics) AddCreated(ctx context.Context, n int64) {
	if m != nil && m.Created != nil {
		m.Created.Add(ctx, n)
	}
}

// AddCompleted records completed trips.
func (m *BusinessMetrics) AddCompleted(ctx context.Context, n int64) {
	if m != nil && m.Completed != nil {
		m.Completed.Add(ctx, n)
	}
}
