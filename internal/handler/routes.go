package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	tripService *service.TripService
	pool        *pgxpool.Pool
}

func NewHandler(tripService *service.TripService, pool *pgxpool.Pool) *Handler {
	return &Handler{
		tripService: tripService,
		pool:        pool,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	_ = params

	var req api.TripData
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
		return
	}

	trip, err := h.tripService.CreateTrip(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.Id.String())
	writeJSON(w, http.StatusCreated, trip)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.GetTrip(r.Context(), tripID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.FinishTrip(r.Context(), tripID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, trip)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTripNotFound):
		writeProblem(w, http.StatusNotFound, "trip_not_found", "Trip not found", err.Error())
	case errors.Is(err, service.ErrTripCompleted):
		writeProblem(w, http.StatusConflict, "trip_completed", "Trip completed", err.Error())
	default:
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	problem := api.Problem{
		Type:   "https://tripgo.example/problems/" + code,
		Title:  title,
		Status: int32(status),
		Code:   code,
	}

	if detail != "" {
		problem.Detail = &detail
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
