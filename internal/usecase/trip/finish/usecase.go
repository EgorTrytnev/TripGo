package finish

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/model"
)

// UseCase completes active trips and records status history.
type UseCase struct {
	repo    tripRepository
	tx      txManager
	counter tripCounter
	logger  *slog.Logger
	now     func() time.Time
}

// New creates the trip completion usecase.
func New(repo tripRepository, tx txManager, counter tripCounter, logger *slog.Logger) *UseCase {
	return &UseCase{repo: repo, tx: tx, counter: counter, logger: logger, now: time.Now}
}

// Execute completes the requested trip inside a transaction.
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (result model.Trip, err error) {
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		var getErr error
		result, getErr = u.repo.GetTripByIDForUpdate(txCtx, id)
		if getErr != nil {
			return getErr
		}
		finishedAt := u.now()
		if err := domaintrip.Finish(&result, finishedAt); err != nil {
			return err
		}
		if err := u.repo.UpdateTripStatus(txCtx, id, result.Status, result.FinishedAt); err != nil {
			return err
		}
		return u.repo.CreateHistory(txCtx, model.TripStatusHistory{TripID: result.ID, FromStatus: domaintrip.StatusActive, ToStatus: domaintrip.StatusCompleted, Reason: "trip finished", ChangedAt: finishedAt})
	})
	if err != nil {
		return model.Trip{}, err
	}
	if u.counter != nil {
		u.counter.AddCompleted(ctx, 1)
	}
	if u.logger != nil {
		u.logger.InfoContext(ctx, "trip completed", "trip_id", result.ID.String())
	}
	return result, nil
}
