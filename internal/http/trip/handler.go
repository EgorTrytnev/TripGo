// Package trip implements the HTTP endpoints for trips.
package trip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/http/domainerror"
	"github.com/EgorTrytnev/TripGo/internal/model"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
)

type createUseCase interface {
	Execute(context.Context, create.Input) (model.Trip, error)
}
type getUseCase interface {
	Execute(context.Context, uuid.UUID) (model.Trip, error)
}
type finishUseCase interface {
	Execute(context.Context, uuid.UUID) (model.Trip, error)
}
type readiness interface{ Ping(context.Context) error }

// Handler implements the HTTP transport for trip and service endpoints.
type Handler struct {
	create       createUseCase
	get          getUseCase
	finish       finishUseCase
	db           readiness
	queryTimeout time.Duration
	logger       *slog.Logger
}

// NewHandler wires the transport to its usecases and readiness dependency.
func NewHandler(create createUseCase, get getUseCase, finish finishUseCase, db readiness, queryTimeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{create: create, get: get, finish: finish, db: db, queryTimeout: queryTimeout, logger: logger}
}

// CreateTrip validates and creates a trip.
func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	_ = params
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		invalid(w, r, "Request body is invalid")
		return
	}
	var req api.TripData
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		invalid(w, r, "Request body is invalid")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		invalid(w, r, "Request body must contain one JSON object")
		return
	}
	if err := validateTripRequest(body, req); err != nil {
		invalid(w, r, err.Error())
		return
	}
	trip, err := h.create.Execute(r.Context(), create.Input{UserID: req.UserId, DriverID: req.DriverId, StartLat: req.StartPoint.Latitude, StartLong: req.StartPoint.Longitude, EndLat: req.EndPoint.Latitude, EndLong: req.EndPoint.Longitude, Price: req.Price})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	response := toResponse(trip)
	w.Header().Set("Location", "/api/v1/trips/"+response.Id.String())
	writeJSON(w, http.StatusCreated, response)
}

// GetTrip retrieves a trip by identifier.
func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	item, err := h.get.Execute(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(item))
}

// FinishTrip completes an active trip.
func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	item, err := h.finish.Execute(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(item))
}

// Health reports whether the process is running.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

// Ready reports whether the service can reach PostgreSQL.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

// OpenAPIErrorHandler writes a consistent response for invalid OpenAPI parameters.
func (h *Handler) OpenAPIErrorHandler(w http.ResponseWriter, r *http.Request, _ error) {
	invalid(w, r, "A request parameter is invalid")
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err)
	if status >= 500 {
		h.logger.ErrorContext(r.Context(), "request failed", "error", err)
	}
	problem.Instance = &r.URL.Path
	writeJSON(w, status, problem)
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

func validateCoordinates(raw json.RawMessage, field string, c api.Coordinates) error {
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
	if c.Latitude < -90 || c.Latitude > 90 {
		return fmt.Errorf("%s.latitude must be between -90 and 90", field)
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return fmt.Errorf("%s.longitude must be between -180 and 180", field)
	}
	return nil
}

func toResponse(t model.Trip) api.Trip {
	return api.Trip{Id: t.ID, UserId: t.UserID, DriverId: t.DriverID, StartPoint: api.Coordinates{Latitude: t.StartLat, Longitude: t.StartLong}, EndPoint: api.Coordinates{Latitude: t.EndLat, Longitude: t.EndLong}, Price: t.Price, Status: api.TripStatus(t.Status), StartedAt: t.StartedAt, FinishedAt: t.FinishedAt}
}

func invalid(w http.ResponseWriter, r *http.Request, detail string) {
	writeProblem(w, r, "invalid_request", "Invalid request", detail)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, r *http.Request, code, title, detail string) {
	p := api.Problem{Type: "https://tripgo.example/problems/" + code, Title: title, Status: int32(http.StatusBadRequest), Detail: &detail, Instance: &r.URL.Path, Code: code}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(p)
}
