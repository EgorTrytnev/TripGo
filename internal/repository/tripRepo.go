package repository

import (
	"context"

	"time"

	"github.com/EgorTrytnev/TripGo/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripRepository struct {
	db *pgxpool.Pool
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip service.TripEntity) error {

}

func (r *TripRepository) GetTripByID(ctx context.Context, id uuid.UUID) (service.TripEntity, error) {

}

func (r *TripRepository) UpdateTripStatus(ctx context.Context, tripID uuid.UUID, status string, finishedAt *time.Time) error {

}
