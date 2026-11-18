package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

// UseCase retrieves trips through its repository contract.
type UseCase struct{ repo tripRepository }

// New creates the trip retrieval usecase.
func New(repo tripRepository) *UseCase { return &UseCase{repo: repo} }

// Execute retrieves one trip by identifier.
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	return u.repo.GetTripByID(ctx, id)
}
