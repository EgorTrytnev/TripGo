// Package get contains the HTTP contract for getting trips.
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

// UseCase is the dependency required by the get endpoint.
type UseCase interface {
	Execute(context.Context, uuid.UUID) (model.Trip, error)
}
