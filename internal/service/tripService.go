package service

import (
	"context"
	"errors"
	"time"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/google/uuid"
)

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

var (
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripCompleted = errors.New("trip already completed")
	ErrDriverBusy    = errors.New("driver already has an active trip")
)

type TripEntity struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLat  float64
	StartLong float64
	EndLat    float64
	EndLong   float64

	Price  int64
	Status string

	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TripStatusHistoryEntity struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus string
	ToStatus   string
	Reason     string
	ChangedAt  time.Time
}

type TxManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip TripEntity) error
	CreateHistory(ctx context.Context, history TripStatusHistoryEntity) error
	GetTripByID(ctx context.Context, id uuid.UUID) (TripEntity, error)
	GetTripByIDForUpdate(ctx context.Context, id uuid.UUID) (TripEntity, error)
	UpdateTripStatus(ctx context.Context, tripID uuid.UUID, status string, finishedAt *time.Time) error
}

type TripService struct {
	repo      TripRepository
	txManager TxManager
}

func NewTripService(repo TripRepository, txManager TxManager) *TripService {
	return &TripService{
		repo:      repo,
		txManager: txManager,
	}
}

func (s *TripService) CreateTrip(ctx context.Context, req api.TripData) (api.Trip, error) {
	trip := TripDataToEntity(req)
	history := TripStatusHistoryToEntity(
		trip.ID,
		"",
		StatusActive,
		"trip created",
	)

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateTrip(txCtx, trip); err != nil {
			return err
		}

		return s.repo.CreateHistory(txCtx, history)
	})
	if err != nil {
		return api.Trip{}, err
	}

	return TripEntityToResponse(trip), nil
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (api.Trip, error) {
	trip, err := s.repo.GetTripByID(ctx, id)
	if err != nil {
		return api.Trip{}, err
	}

	return TripEntityToResponse(trip), nil
}

func (s *TripService) FinishTrip(ctx context.Context, id uuid.UUID) (api.Trip, error) {
	var trip TripEntity

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		var err error
		trip, err = s.repo.GetTripByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		if trip.Status == StatusCompleted {
			return ErrTripCompleted
		}

		finishedAt := time.Now()
		if err := s.repo.UpdateTripStatus(txCtx, id, StatusCompleted, &finishedAt); err != nil {
			return err
		}

		history := TripStatusHistoryToEntity(
			trip.ID,
			trip.Status,
			StatusCompleted,
			"trip finished",
		)
		if err := s.repo.CreateHistory(txCtx, history); err != nil {
			return err
		}

		trip.Status = StatusCompleted
		trip.FinishedAt = &finishedAt
		trip.UpdatedAt = finishedAt

		return nil
	})
	if err != nil {
		return api.Trip{}, err
	}

	return TripEntityToResponse(trip), nil
}
