package position

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/model"
)

// Input содержит координату, полученную от устройства водителя.
type Input struct {
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

// UseCase сохраняет координаты и возвращает маршрут поездки.
type UseCase struct {
	repo tripRepository
	tx   txManager
	now  func() time.Time
}

// New создаёт usecase для работы с координатами.
func New(repo tripRepository, tx txManager) *UseCase {
	return &UseCase{repo: repo, tx: tx, now: time.Now}
}

// Create сохраняет координату активной поездки в одной транзакции.
func (u *UseCase) Create(ctx context.Context, in Input) (model.Position, error) {
	if err := domaintrip.ValidatePosition(in.Latitude, in.Longitude, in.RecordedAt, u.now()); err != nil {
		return model.Position{}, err
	}
	var result model.Position
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		trip, err := u.repo.GetTripByIDForUpdate(txCtx, in.TripID)
		if err != nil {
			return err
		}
		if trip.Status != domaintrip.StatusActive {
			return domaintrip.ErrCompleted
		}
		result, err = u.repo.CreatePosition(txCtx, model.Position{TripID: in.TripID, Latitude: in.Latitude, Longitude: in.Longitude, RecordedAt: in.RecordedAt})
		if err != nil {
			return err
		}
		return u.repo.UpdateLastPositionAt(txCtx, in.TripID, in.RecordedAt)
	})
	if err != nil {
		return model.Position{}, err
	}
	return result, nil
}

// List возвращает все координаты поездки в порядке записи на устройстве.
func (u *UseCase) List(ctx context.Context, tripID uuid.UUID) ([]model.Position, error) {
	if _, err := u.repo.GetTripByID(ctx, tripID); err != nil {
		return nil, err
	}
	return u.repo.GetPositionsByTripID(ctx, tripID)
}
