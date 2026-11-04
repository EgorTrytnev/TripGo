package create

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/model"
)

// Input contains the validated fields needed to create a trip.
type Input struct {
	UserID              uuid.UUID
	DriverID            uuid.UUID
	StartLat, StartLong float64
	EndLat, EndLong     float64
	Price               int64
}

// UseCase creates trips and their initial history records transactionally.
type UseCase struct {
	repo    tripRepository
	tx      txManager
	counter tripCounter
	logger  *slog.Logger
	now     func() time.Time
}

// New creates the trip creation usecase.
func New(repo tripRepository, tx txManager, counter tripCounter, logger *slog.Logger) *UseCase {
	return &UseCase{repo: repo, tx: tx, counter: counter, logger: logger, now: time.Now}
}

// Execute creates a trip and records its initial status.
func (u *UseCase) Execute(ctx context.Context, in Input) (model.Trip, error) {
	now := u.now()
	t := model.Trip{ID: uuid.New(), UserID: in.UserID, DriverID: in.DriverID, StartLat: in.StartLat, StartLong: in.StartLong, EndLat: in.EndLat, EndLong: in.EndLong, Price: in.Price, Status: trip.StatusActive, StartedAt: now, CreatedAt: now, UpdatedAt: now}
	h := model.TripStatusHistory{TripID: t.ID, ToStatus: trip.StatusActive, Reason: "trip created", ChangedAt: now}
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.repo.CreateTrip(txCtx, t); err != nil {
			return err
		}
		return u.repo.CreateHistory(txCtx, h)
	})
	if err != nil {
		return model.Trip{}, err
	}
	if u.counter != nil {
		u.counter.AddCreated(ctx, 1)
	}
	if u.logger != nil {
		u.logger.InfoContext(ctx, "trip created", "trip_id", t.ID.String())
	}
	return t, nil
}
