// Package finish contains the HTTP contract for finishing trips.
package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

// UseCase is the dependency required by the finish endpoint.
type UseCase interface {
	Execute(context.Context, uuid.UUID) (model.Trip, error)
}
