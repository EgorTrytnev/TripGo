package service

import (
	"time"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/google/uuid"
)

func TripDataToEntity(req api.TripData) TripEntity {
	now := time.Now()

	return TripEntity{
		ID:        uuid.New(),
		UserID:    req.UserId,
		DriverID:  req.DriverId,
		StartLat:  req.StartPoint.Latitude,
		StartLong: req.StartPoint.Longitude,
		EndLat:    req.EndPoint.Latitude,
		EndLong:   req.EndPoint.Longitude,
		Price:     req.Price,
		Status:    "active",
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TripEntityToResponse(t TripEntity) api.Trip {
	return api.Trip{
		Id:       api.TripId(t.ID),
		UserId:   api.TripId(t.UserID),
		DriverId: api.TripId(t.DriverID),
		StartPoint: api.Coordinates{
			Latitude:  t.StartLat,
			Longitude: t.StartLong,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndLat,
			Longitude: t.EndLong,
		},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

func TripStatusHistoryToEntity(tripID uuid.UUID, fromStatus string, toStatus string, reason string) TripStatusHistoryEntity {
	return TripStatusHistoryEntity{
		TripID:     tripID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Reason:     reason,
		ChangedAt:  time.Now(),
	}
}
