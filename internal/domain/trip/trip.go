package trip

import (
	"time"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

const (
	// StatusActive marks a trip that has not been completed.
	StatusActive = "active"
	// StatusCompleted marks a finished trip.
	StatusCompleted = "completed"
)

// Finish applies the only valid terminal state transition for a trip.
func Finish(t *model.Trip, at time.Time) error {
	if t.Status != StatusActive {
		return ErrCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at
	return nil
}
