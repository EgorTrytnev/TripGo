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

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/http/domainerror"
	createhttp "github.com/EgorTrytnev/TripGo/internal/http/trip/create"
	finishhttp "github.com/EgorTrytnev/TripGo/internal/http/trip/finish"
	gethttp "github.com/EgorTrytnev/TripGo/internal/http/trip/get"
	positionhttp "github.com/EgorTrytnev/TripGo/internal/http/trip/position"
	"github.com/EgorTrytnev/TripGo/internal/model"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
	positionusecase "github.com/EgorTrytnev/TripGo/internal/usecase/trip/position"
)

type readiness interface{ Ping(context.Context) error }

// Handler implements the HTTP transport for trip and service endpoints.
type Handler struct {
	create         createhttp.UseCase
	get            gethttp.UseCase
	finish         finishhttp.UseCase
	positions      positionhttp.UseCase
	positionCancel positionhttp.Canceler
	db             readiness
	queryTimeout   time.Duration
	logger         *slog.Logger
}

func (h *Handler) SetPositionCanceler(canceler positionhttp.Canceler) { h.positionCancel = canceler }

// NewHandler wires the transport to its usecases and readiness dependency.
func NewHandler(create createhttp.UseCase, get gethttp.UseCase, finish finishhttp.UseCase, db readiness, queryTimeout time.Duration, logger *slog.Logger, positions ...positionhttp.UseCase) *Handler {
	var positionUC positionhttp.UseCase
	if len(positions) > 0 {
		positionUC = positions[0]
	}
	return &Handler{create: create, get: get, finish: finish, positions: positionUC, db: db, queryTimeout: queryTimeout, logger: logger}
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
	if err := validateJSONFields(body); err != nil {
		invalid(w, r, err.Error())
		return
	}
	if err := domaintrip.Validate(req.UserId, req.DriverId, req.StartPoint.Latitude, req.StartPoint.Longitude, req.EndPoint.Latitude, req.EndPoint.Longitude, req.Price); err != nil {
		h.writeServiceError(w, r, err)
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

// CreateTripPosition сохраняет одну точку маршрута.
func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	if h.positions == nil {
		invalid(w, r, "Position usecase is not configured")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		invalid(w, r, "Request body is invalid")
		return
	}
	var req api.PositionData
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
	var fields struct {
		Coordinates *struct {
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
		} `json:"coordinates"`
		RecordedAt *time.Time `json:"recorded_at"`
	}
	if err := json.Unmarshal(body, &fields); err != nil || fields.Coordinates == nil || fields.Coordinates.Latitude == nil || fields.Coordinates.Longitude == nil || fields.RecordedAt == nil {
		invalid(w, r, "coordinates, latitude, longitude and recorded_at are required")
		return
	}
	item, err := h.positions.Create(r.Context(), positionusecase.Input{TripID: id, Latitude: req.Coordinates.Latitude, Longitude: req.Coordinates.Longitude, RecordedAt: req.RecordedAt})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	if h.positionCancel != nil {
		h.positionCancel.Cancel(id)
	}
	writeJSON(w, http.StatusCreated, toPositionResponse(item))
}

// ListTripPositions возвращает маршрут поездки.
func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, id api.TripId) {
	items, err := h.positions.List(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	response := api.TripPositionsResponse{Positions: make([]api.TripPosition, 0, len(items))}
	for _, item := range items {
		response.Positions = append(response.Positions, toPositionResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func toPositionResponse(p model.Position) api.TripPosition {
	return api.TripPosition{Id: p.ID, TripId: p.TripID, Coordinates: api.Coordinates{Latitude: p.Latitude, Longitude: p.Longitude}, RecordedAt: p.RecordedAt}
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
	writeProblemResponse(w, status, problem)
}

func validateJSONFields(body []byte) error {
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
	for _, point := range []struct {
		raw   json.RawMessage
		field string
	}{{fields["start_point"], "start_point"}, {fields["end_point"], "end_point"}} {
		var pointFields map[string]json.RawMessage
		if err := json.Unmarshal(point.raw, &pointFields); err != nil || pointFields == nil {
			return errors.New(point.field + " must be an object")
		}
		for _, key := range []string{"latitude", "longitude"} {
			value, ok := pointFields[key]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return errors.New(point.field + "." + key + " is required")
			}
		}
	}
	return nil
}

func toResponse(t model.Trip) api.Trip {
	return api.Trip{Id: t.ID, UserId: t.UserID, DriverId: t.DriverID, StartPoint: api.Coordinates{Latitude: t.StartLat, Longitude: t.StartLong}, EndPoint: api.Coordinates{Latitude: t.EndLat, Longitude: t.EndLong}, Price: t.Price, Status: api.TripStatus(t.Status), StartedAt: t.StartedAt, FinishedAt: t.FinishedAt, LastPositionAt: t.LastPositionAt}
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
	writeProblemResponse(w, http.StatusBadRequest, p)
}

func writeProblemResponse(w http.ResponseWriter, status int, p api.Problem) {
	if status < 0 || status > 999 {
		status = http.StatusInternalServerError
	}
	p.Status = int32(status)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}
