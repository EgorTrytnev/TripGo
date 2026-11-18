// Package finish implements trip completion.
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen -source contract.go -destination mocks_test.go -package finish_test
type tripRepository interface {
	GetTripByIDForUpdate(context.Context, uuid.UUID) (model.Trip, error)
	UpdateTripStatus(context.Context, uuid.UUID, string, *time.Time) error
	CreateHistory(context.Context, model.TripStatusHistory) error
}
type txManager interface {
	Do(context.Context, func(context.Context) error) error
}
type tripCounter interface{ AddCompleted(context.Context, int64) }
