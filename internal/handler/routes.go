package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	tripService  *service.TripService
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewHandler(tripService *service.TripService, pool *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{
		tripService:  tripService,
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	_ = params // Idempotency-Key is optional bonus work.

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request body is invalid")
		return
	}

	var req api.TripData
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request body is invalid")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Request body must contain one JSON object")
		return
	}
	if err := validateTripRequest(body, req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
		return
	}

	trip, err := h.tripService.CreateTrip(r.Context(), req)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.Id.String())
	writeJSON(w, http.StatusCreated, trip)
}

func validateTripRequest(body []byte, req api.TripData) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return errors.New("request body must be a JSON object")
	}
	for _, key := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("field %q is required", key)
		}
	}
	if req.UserId == uuid.Nil || req.DriverId == uuid.Nil {
		return errors.New("user_id and driver_id must be non-zero UUIDs")
	}
	if err := validateCoordinates(fields["start_point"], "start_point", req.StartPoint); err != nil {
		return err
	}
	if err := validateCoordinates(fields["end_point"], "end_point", req.EndPoint); err != nil {
		return err
	}
	if req.Price < 0 {
		return errors.New("price must be greater than or equal to zero")
	}
	return nil
}

func validateCoordinates(raw json.RawMessage, field string, coordinates api.Coordinates) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return fmt.Errorf("%s must be an object", field)
	}
	for _, key := range []string{"latitude", "longitude"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s.%s is required", field, key)
		}
	}
	if coordinates.Latitude < -90 || coordinates.Latitude > 90 {
		return fmt.Errorf("%s.latitude must be between -90 and 90", field)
	}
	if coordinates.Longitude < -180 || coordinates.Longitude > 180 {
		return fmt.Errorf("%s.longitude must be between -180 and 180", field)
	}
	return nil
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.GetTrip(r.Context(), tripID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.FinishTrip(r.Context(), tripID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

// OpenAPIErrorHandler keeps malformed path parameters in the same error format as other API errors.
func (h *Handler) OpenAPIErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	_ = err
	writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "A request parameter is invalid")
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrTripNotFound):
		writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "No trip exists with this id")
	case errors.Is(err, service.ErrTripCompleted):
		writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip completed", "The trip has already been completed")
	case errors.Is(err, service.ErrDriverBusy):
		writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
	default:
		log.Printf("request %s failed: %v", r.URL.Path, err)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "An unexpected error occurred")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/" + code,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &r.URL.Path,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
