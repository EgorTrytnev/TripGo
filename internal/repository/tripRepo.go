// Package repository contains PostgreSQL-backed trip storage.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	db "github.com/EgorTrytnev/TripGo/internal/infra/postgres"
	"github.com/EgorTrytnev/TripGo/internal/model"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// TripRepository stores and retrieves trips from PostgreSQL.
type TripRepository struct {
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

// DBContext is the subset of pgx operations used by the repository.
type DBContext interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewTripRepository creates a PostgreSQL trip repository.
func NewTripRepository(db *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{db: db, queryTimeout: queryTimeout}
}

func (r *TripRepository) executor(ctx context.Context) DBContext {
	if tx, ok := db.TxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

// CreateTrip inserts a trip row.
func (r *TripRepository) CreateTrip(ctx context.Context, trip model.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)

	q, args, err := psql.Insert("trips").Columns(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status",
		"started_at", "finished_at", "last_position_at", "created_at", "updated_at").
		Values(
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartLat, trip.StartLong, trip.EndLat, trip.EndLong,
			trip.Price, trip.Status,
			trip.StartedAt, trip.FinishedAt, trip.LastPositionAt, trip.CreatedAt, trip.UpdatedAt).ToSql()
	if err != nil {
		return fmt.Errorf("create sql error: %w", err)
	}

	_, err = exec.Exec(ctx, q, args...)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver_idx" {
			return fmt.Errorf("create trip: %w", domaintrip.ErrDriverBusy)
		}
		return fmt.Errorf("create exec error: %w", err)
	}

	return nil
}

// GetTripByID returns a trip by identifier.
func (r *TripRepository) GetTripByID(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	return r.getTripByID(ctx, id, false)
}

// GetTripByIDForUpdate returns and locks a trip row in the current transaction.
func (r *TripRepository) GetTripByIDForUpdate(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	return r.getTripByID(ctx, id, true)
}

func (r *TripRepository) getTripByID(ctx context.Context, id uuid.UUID, forUpdate bool) (model.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)
	queryBuilder := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status",
		"started_at", "finished_at", "last_position_at", "created_at", "updated_at").
		From("trips").Where(sq.Eq{"id": id})
	if forUpdate {
		queryBuilder = queryBuilder.Suffix("FOR UPDATE")
	}
	q, args, err := queryBuilder.ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("get sql error: %w", err)
	}
	var trip model.Trip
	err = exec.QueryRow(ctx, q, args...).Scan(
		&trip.ID, &trip.UserID, &trip.DriverID,
		&trip.StartLat, &trip.StartLong, &trip.EndLat, &trip.EndLong,
		&trip.Price, &trip.Status,
		&trip.StartedAt, &trip.FinishedAt, &trip.LastPositionAt, &trip.CreatedAt, &trip.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Trip{}, domaintrip.ErrNotFound
		}
		return model.Trip{}, fmt.Errorf("get scan error: %w", err)
	}

	return trip, nil
}

// CreatePosition stores one recorded route point.
func (r *TripRepository) CreatePosition(ctx context.Context, position model.Position) (model.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	q, args, err := psql.Insert("trip_positions").Columns(
		"trip_id", "latitude", "longitude", "recorded_at",
	).Values(
		position.TripID, position.Latitude, position.Longitude, position.RecordedAt,
	).Suffix("RETURNING id, created_at").ToSql()
	if err != nil {
		return model.Position{}, fmt.Errorf("create position sql error: %w", err)
	}
	if err := r.executor(ctx).QueryRow(ctx, q, args...).Scan(&position.ID, &position.CreatedAt); err != nil {
		return model.Position{}, fmt.Errorf("create position error: %w", err)
	}
	return position, nil
}

// UpdateLastPositionAt moves the trip timestamp only when the new point is newer.
func (r *TripRepository) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	q, args, err := psql.Update("trips").
		Set("last_position_at", recordedAt).
		Where(sq.Eq{"id": tripID}).
		Where(sq.Or{sq.Eq{"last_position_at": nil}, sq.Lt{"last_position_at": recordedAt}}).
		ToSql()
	if err != nil {
		return fmt.Errorf("update position time sql error: %w", err)
	}
	if _, err := r.executor(ctx).Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("update position time error: %w", err)
	}
	return nil
}

// GetPositionsByTripID returns route points in recorded order.
func (r *TripRepository) GetPositionsByTripID(ctx context.Context, tripID uuid.UUID) ([]model.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	q, args, err := psql.Select("id", "trip_id", "latitude", "longitude", "recorded_at", "created_at").
		From("trip_positions").Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").ToSql()
	if err != nil {
		return nil, fmt.Errorf("get positions sql error: %w", err)
	}
	rows, err := r.executor(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get positions query error: %w", err)
	}
	defer rows.Close()
	positions := make([]model.Position, 0)
	for rows.Next() {
		var position model.Position
		if err := rows.Scan(&position.ID, &position.TripID, &position.Latitude, &position.Longitude, &position.RecordedAt, &position.CreatedAt); err != nil {
			return nil, fmt.Errorf("get position scan error: %w", err)
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get positions rows error: %w", err)
	}
	return positions, nil
}

// FindStaleTrips returns active trips ordered from the oldest position update.
func (r *TripRepository) FindStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	lastPosition := "COALESCE(last_position_at, started_at)"
	q, args, err := psql.Select("id", "driver_id", lastPosition).
		From("trips").
		Where("status = 'active'").
		Where(sq.Expr(lastPosition+" < now() - (? * interval '1 second')", staleAfter.Seconds())).
		OrderBy(lastPosition+" ASC", "id ASC").ToSql()
	if err != nil {
		return nil, fmt.Errorf("find stale trips sql error: %w", err)
	}
	rows, err := r.executor(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("find stale trips query error: %w", err)
	}
	defer rows.Close()
	trips := make([]model.StaleTrip, 0)
	for rows.Next() {
		var trip model.StaleTrip
		if err := rows.Scan(&trip.ID, &trip.DriverID, &trip.LastPositionAt); err != nil {
			return nil, fmt.Errorf("find stale trip scan error: %w", err)
		}
		trips = append(trips, trip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find stale trips rows error: %w", err)
	}
	return trips, nil
}

// UpdateTripStatus updates an active trip's status and timestamps.
func (r *TripRepository) UpdateTripStatus(ctx context.Context, tripID uuid.UUID, status string, finishedAt *time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	exec := r.executor(ctx)
	q, args, err := psql.Update("trips").
		Set("status", status).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": tripID, "status": domaintrip.StatusActive}).ToSql()
	if err != nil {
		return fmt.Errorf("update sql error: %w", err)
	}

	ret, err := exec.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if ret.RowsAffected() == 0 {
		return domaintrip.ErrCompleted
	}

	return nil
}

// CreateHistory inserts a status history row.
func (r *TripRepository) CreateHistory(ctx context.Context, history model.TripStatusHistory) error {
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
