// Пакет position содержит сценарии работы с координатами поездки.
package position

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen -source contract.go -destination mocks_test.go -package position_test
type tripRepository interface {
	GetTripByID(context.Context, uuid.UUID) (model.Trip, error)
	GetTripByIDForUpdate(context.Context, uuid.UUID) (model.Trip, error)
	CreatePosition(context.Context, model.Position) (model.Position, error)
	UpdateLastPositionAt(context.Context, uuid.UUID, time.Time) error
	GetPositionsByTripID(context.Context, uuid.UUID) ([]model.Position, error)
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}
