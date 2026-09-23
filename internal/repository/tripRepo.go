package repository

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/EgorTrytnev/TripGo/internal/service"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var ErrTripNotFound = errors.New("trip not found")

type TripRepository struct {
	db *pgxpool.Pool
}

func NewTripRepository(db *pgxpool.Pool) *TripRepository {
	return &TripRepository{db: db}
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip service.TripEntity) error {

	q, args, err := psql.Insert("trips").Columns(
		"id", "user_id", "driver_id",
		"start_lat", "start_long", "end_lat", "end_long",
		"price", "status",
		"started_at", "finished_at", "created_at", "updated_at").
		Values(
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartLat, trip.StartLong, trip.EndLat, trip.EndLong,
			trip.Price, trip.Status,
			trip.StartedAt, trip.FinishedAt, trip.CreatedAt, trip.UpdatedAt).ToSql()

	if err != nil {
		return fmt.Errorf("create sql error: %w", err)
	}

	_, err = r.db.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("create exec error: %w", err)
	}

	return nil

}

func (r *TripRepository) GetTripByID(ctx context.Context, id uuid.UUID) (service.TripEntity, error) {

	q, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_lat", "start_long", "end_lat", "end_long",
		"price", "status",
		"started_at", "finished_at", "created_at", "updated_at").
		From("trips").Where(sq.Eq{"id": id}).ToSql()

	if err != nil {
		return service.TripEntity{}, fmt.Errorf("get sql error: %w", err)
	}
	var trip service.TripEntity
	err = r.db.QueryRow(ctx, q, args...).Scan(
		&trip.ID, &trip.UserID, &trip.DriverID,
		&trip.StartLat, &trip.StartLong, &trip.EndLat, &trip.EndLong,
		&trip.Price, &trip.Status,
		&trip.StartedAt, &trip.FinishedAt, &trip.CreatedAt, &trip.UpdatedAt)

	if err != nil {
		return service.TripEntity{}, fmt.Errorf("get scan error: %w", err)
	}

	return trip, nil

}

func (r *TripRepository) UpdateTripStatus(ctx context.Context, tripID uuid.UUID, status string, finishedAt *time.Time) error {
	q, args, err := psql.Update("trips").
		Set("status", status).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": tripID}).ToSql()

	if err != nil {
		return fmt.Errorf("update sql error: %w", err)
	}

	ret, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if ret.RowsAffected() == 0 {
		return ErrTripNotFound
	}

	return nil
}
