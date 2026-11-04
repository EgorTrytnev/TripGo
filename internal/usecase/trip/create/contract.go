// Package create implements trip creation.
package create

import (
	"context"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen -source contract.go -destination mocks_test.go -package create_test
type tripRepository interface {
	CreateTrip(context.Context, model.Trip) error
	CreateHistory(context.Context, model.TripStatusHistory) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type tripCounter interface{ AddCreated(context.Context, int64) }
