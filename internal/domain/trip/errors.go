// Package trip contains trip state rules and domain errors.
package trip

import (
	"errors"
	"time"
)

// ValidatePosition проверяет полученную координату поездки.
func ValidatePosition(latitude, longitude float64, recordedAt, now time.Time) error {
	if latitude < -90 || latitude > 90 {
		return ValidationError{Field: "coordinates.latitude", Message: "coordinates.latitude must be between -90 and 90"}
	}
	if longitude < -180 || longitude > 180 {
		return ValidationError{Field: "coordinates.longitude", Message: "coordinates.longitude must be between -180 and 180"}
	}
	if recordedAt.IsZero() {
		return ValidationError{Field: "recorded_at", Message: "recorded_at is required"}
	}
	if recordedAt.After(now.Add(time.Minute)) {
		return ValidationError{Field: "recorded_at", Message: "recorded_at must not be more than one minute in the future"}
	}
	return nil
}

// ValidationError описывает ошибку конкретного поля.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string { return e.Message }

// Validate проверяет правила поездки.
func Validate(userID, driverID [16]byte, startLat, startLong, endLat, endLong float64, price int64) error {
	if userID == [16]byte{} {
		return ValidationError{Field: "user_id", Message: "user_id must be a non-zero UUID"}
	}
	if driverID == [16]byte{} {
		return ValidationError{Field: "driver_id", Message: "driver_id must be a non-zero UUID"}
	}
	if startLat < -90 || startLat > 90 {
		return ValidationError{Field: "start_point.latitude", Message: "start_point.latitude must be between -90 and 90"}
	}
	if startLong < -180 || startLong > 180 {
		return ValidationError{Field: "start_point.longitude", Message: "start_point.longitude must be between -180 and 180"}
	}
	if endLat < -90 || endLat > 90 {
		return ValidationError{Field: "end_point.latitude", Message: "end_point.latitude must be between -90 and 90"}
	}
	if endLong < -180 || endLong > 180 {
		return ValidationError{Field: "end_point.longitude", Message: "end_point.longitude must be between -180 and 180"}
	}
	if price < 0 {
		return ValidationError{Field: "price", Message: "price must be greater than or equal to zero"}
	}
	return nil
}

var (
	// ErrNotFound means that the requested trip does not exist.
	ErrNotFound = errors.New("trip not found")
	// ErrCompleted means that the requested trip is already finished.
	ErrCompleted = errors.New("trip already completed")
	// ErrDriverBusy means that the driver already has an active trip.
	ErrDriverBusy = errors.New("driver already has an active trip")
)
