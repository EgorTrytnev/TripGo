package model

import (
	"time"

	"github.com/google/uuid"
)

// Position is a recorded point of a trip route.
type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}

// StaleTrip is the data needed to request a fresh position from a driver.
type StaleTrip struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	LastPositionAt time.Time
}
