package repository

import (
	"context"
	"fmt"
	"time"

	db "github.com/EgorTrytnev/TripGo/internal/db"
	"github.com/EgorTrytnev/TripGo/internal/service"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type TripRepository struct {
	db *pgxpool.Pool
}

type DBContext interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewTripRepository(db *pgxpool.Pool) *TripRepository {
	return &TripRepository{db: db}
}

func (r *TripRepository) executor(ctx context.Context) DBContext {
	if tx, ok := db.TxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip service.TripEntity) error {
	exec := r.executor(ctx)

	q, args, err := psql.Insert("trips").Columns(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
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

	_, err = exec.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("create exec error: %w", err)
	}

	return nil
}

func (r *TripRepository) GetTripByID(ctx context.Context, id uuid.UUID) (service.TripEntity, error) {
	exec := r.executor(ctx)
	q, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status",
		"started_at", "finished_at", "created_at", "updated_at").
		From("trips").Where(sq.Eq{"id": id}).ToSql()

	if err != nil {
		return service.TripEntity{}, fmt.Errorf("get sql error: %w", err)
	}
	var trip service.TripEntity
	err = exec.QueryRow(ctx, q, args...).Scan(
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
	exec := r.executor(ctx)
	q, args, err := psql.Update("trips").
		Set("status", status).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": tripID}).ToSql()

	if err != nil {
		return fmt.Errorf("update sql error: %w", err)
	}

	ret, err := exec.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if ret.RowsAffected() == 0 {
		return service.ErrTripNotFound
	}

	return nil
}

func (r *TripRepository) CreateHistory(ctx context.Context, history service.TripStatusHistoryEntity) error {
	exec := r.executor(ctx)

	q, args, err := psql.Insert("trip_status_history").Columns(
		"trip_id",
		"from_status",
		"to_status",
		"reason",
		"changed_at",
	).Values(
		history.TripID,
		history.FromStatus,
		history.ToStatus,
		history.Reason,
		history.ChangedAt,
	).ToSql()
	if err != nil {
		return fmt.Errorf("create history sql error: %w", err)
	}

	if _, err := exec.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("create history exec error: %w", err)
	}

	return nil
}
