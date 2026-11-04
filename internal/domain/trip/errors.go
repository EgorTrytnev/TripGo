// Package trip contains trip state rules and domain errors.
package trip

import "errors"

var (
	// ErrNotFound means that the requested trip does not exist.
	ErrNotFound = errors.New("trip not found")
	// ErrCompleted means that the requested trip is already finished.
	ErrCompleted = errors.New("trip already completed")
	// ErrDriverBusy means that the driver already has an active trip.
	ErrDriverBusy = errors.New("driver already has an active trip")
)
