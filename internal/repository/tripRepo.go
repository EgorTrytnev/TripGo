package repository

import (
	"context"
	"errors"
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
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

type DBContext interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewTripRepository(db *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{db: db, queryTimeout: queryTimeout}
}

func (r *TripRepository) executor(ctx context.Context) DBContext {
	if tx, ok := db.TxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip service.TripEntity) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
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
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx" {
			return fmt.Errorf("create trip: %w", service.ErrDriverBusy)
		}
		return fmt.Errorf("create exec error: %w", err)
	}

	return nil
}

func (r *TripRepository) GetTripByID(ctx context.Context, id uuid.UUID) (service.TripEntity, error) {
	return r.getTripByID(ctx, id, false)
}

func (r *TripRepository) GetTripByIDForUpdate(ctx context.Context, id uuid.UUID) (service.TripEntity, error) {
	return r.getTripByID(ctx, id, true)
}

func (r *TripRepository) getTripByID(ctx context.Context, id uuid.UUID, forUpdate bool) (service.TripEntity, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)
	queryBuilder := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status",
		"started_at", "finished_at", "created_at", "updated_at").
		From("trips").Where(sq.Eq{"id": id})
	if forUpdate {
		queryBuilder = queryBuilder.Suffix("FOR UPDATE")
	}
	q, args, err := queryBuilder.ToSql()

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
		if errors.Is(err, pgx.ErrNoRows) {
			return service.TripEntity{}, service.ErrTripNotFound
		}
		return service.TripEntity{}, fmt.Errorf("get scan error: %w", err)
	}

	return trip, nil

}

func (r *TripRepository) UpdateTripStatus(ctx context.Context, tripID uuid.UUID, status string, finishedAt *time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)
	q, args, err := psql.Update("trips").
		Set("status", status).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": tripID, "status": service.StatusActive}).ToSql()

	if err != nil {
		return fmt.Errorf("update sql error: %w", err)
	}

	ret, err := exec.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if ret.RowsAffected() == 0 {
		return service.ErrTripCompleted
	}

	return nil
}

func (r *TripRepository) CreateHistory(ctx context.Context, history service.TripStatusHistoryEntity) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)
	var fromStatus any
	if history.FromStatus != "" {
		fromStatus = history.FromStatus
	}

	q, args, err := psql.Insert("trip_status_history").Columns(
		"trip_id",
		"from_status",
		"to_status",
		"reason",
		"changed_at",
	).Values(
		history.TripID,
		fromStatus,
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
