// Package domainerror maps domain errors to public HTTP responses.
package domainerror

import (
	"errors"
	"net/http"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	api "github.com/EgorTrytnev/TripGo/internal/generated"
)

// Problem converts a domain error to its public HTTP status and Problem Details body.
func Problem(err error) (int, api.Problem) {
	status, statusCode, code, title, detail := http.StatusInternalServerError, int32(http.StatusInternalServerError), "internal_error", "Internal Server Error", "An unexpected error occurred"
	switch {
	case errors.Is(err, domaintrip.ErrNotFound):
		status, statusCode, code, title, detail = http.StatusNotFound, int32(http.StatusNotFound), "trip_not_found", "Trip not found", "No trip exists with this id"
	case errors.Is(err, domaintrip.ErrCompleted):
		status, statusCode, code, title, detail = http.StatusConflict, int32(http.StatusConflict), "trip_completed", "Trip completed", "The trip has already been completed"
	case errors.Is(err, domaintrip.ErrDriverBusy):
		status, statusCode, code, title, detail = http.StatusConflict, int32(http.StatusConflict), "driver_busy", "Driver busy", "Driver already has an active trip"
	}
	typ, inst := "https://tripgo.example/problems/"+code, ""
	return status, api.Problem{Type: typ, Title: title, Status: statusCode, Detail: &detail, Instance: &inst, Code: code}
}
