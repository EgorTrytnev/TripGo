package model

import (
	"time"

	"github.com/google/uuid"
)

// Position хранит одну точку маршрута поездки.
type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}

// StaleTrip содержит данные поездки, для которой нужно запросить координаты.
type StaleTrip struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	LastPositionAt time.Time
}
