package trip

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	api "github.com/EgorTrytnev/TripGo/internal/generated"
	"github.com/EgorTrytnev/TripGo/internal/model"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
)

type createStub struct {
	called bool
	err    error
	trip   model.Trip
}

func (s *createStub) Execute(context.Context, create.Input) (model.Trip, error) {
	s.called = true
	return s.trip, s.err
}

type getStub struct{}

func (getStub) Execute(context.Context, uuid.UUID) (model.Trip, error) { return model.Trip{}, nil }

type finishStub struct{}

func (finishStub) Execute(context.Context, uuid.UUID) (model.Trip, error) { return model.Trip{}, nil }

type readyStub struct{}

func (readyStub) Ping(context.Context) error { return nil }

func TestCreateTripRejectsInvalidRequest(t *testing.T) {
	createUC := &createStub{}
	h := NewHandler(createUC, getStub{}, finishStub{}, readyStub{}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(`{"price":-1}`))
	rec := httptest.NewRecorder()
	h.CreateTrip(rec, req, api.CreateTripParams{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if createUC.called {
		t.Fatal("invalid request reached usecase")
	}
}

func TestCreateTripReturnsCreatedTrip(t *testing.T) {
	id := uuid.New()
	created := model.Trip{ID: id, UserID: uuid.New(), DriverID: uuid.New(), Status: "active", StartedAt: time.Now().UTC()}
	createUC := &createStub{trip: created}
	h := NewHandler(createUC, getStub{}, finishStub{}, readyStub{}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"user_id":"` + created.UserID.String() + `","driver_id":"` + created.DriverID.String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.4},"price":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreateTrip(rec, req, api.CreateTripParams{})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, want 201: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/api/v1/trips/"+id.String() {
		t.Fatalf("unexpected Location header: %q", rec.Header().Get("Location"))
	}
	if !createUC.called {
		t.Fatal("valid request did not reach usecase")
	}
}
