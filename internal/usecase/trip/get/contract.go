// Package get implements trip retrieval.
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen -source contract.go -destination mocks_test.go -package get_test
type tripRepository interface {
	GetTripByID(context.Context, uuid.UUID) (model.Trip, error)
}
