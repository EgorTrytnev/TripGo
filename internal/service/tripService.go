package service

import (
	"time"

	"github.com/google/uuid"
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
