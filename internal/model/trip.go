// Package model defines structures shared by application layers.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Trip is the persistence and usecase representation of a trip.
type Trip struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartLat   float64
	StartLong  float64
	EndLat     float64
	EndLong    float64
	Price      int64
	Status     string
	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TripStatusHistory records a trip status transition.
type TripStatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus string
	ToStatus   string
	Reason     string
	ChangedAt  time.Time
}
