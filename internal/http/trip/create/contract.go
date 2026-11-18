// Package create contains the HTTP contract for creating trips.
package create

import (
	"context"

	"github.com/EgorTrytnev/TripGo/internal/model"
	usecase "github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
)

// UseCase is the dependency required by the create endpoint.
type UseCase interface {
	Execute(context.Context, usecase.Input) (model.Trip, error)
}
