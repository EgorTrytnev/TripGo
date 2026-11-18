package finish_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/model"
	finishtrip "github.com/EgorTrytnev/TripGo/internal/usecase/trip/finish"
)

func TestFinishActiveTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	counter := NewMocktripCounter(ctrl)
	id := uuid.New()
	repo.EXPECT().GetTripByIDForUpdate(gomock.Any(), id).Return(model.Trip{ID: id, Status: domaintrip.StatusActive}, nil)
	repo.EXPECT().UpdateTripStatus(gomock.Any(), id, domaintrip.StatusCompleted, gomock.Any()).Return(nil)
	repo.EXPECT().CreateHistory(gomock.Any(), gomock.Any()).Return(nil)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(c context.Context, fn func(context.Context) error) error { return fn(c) })
	counter.EXPECT().AddCompleted(gomock.Any(), int64(1))
	got, err := finishtrip.New(repo, tx, counter, slog.New(slog.NewTextHandler(io.Discard, nil))).Execute(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domaintrip.StatusCompleted || got.FinishedAt == nil {
		t.Fatalf("trip was not completed: %+v", got)
	}
}

func TestFinishCompletedTripReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	counter := NewMocktripCounter(ctrl)
	id := uuid.New()
	repo.EXPECT().GetTripByIDForUpdate(gomock.Any(), id).Return(model.Trip{ID: id, Status: domaintrip.StatusCompleted}, nil)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(c context.Context, fn func(context.Context) error) error { return fn(c) })
	_, err := finishtrip.New(repo, tx, counter, slog.New(slog.NewTextHandler(io.Discard, nil))).Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrCompleted) {
		t.Fatalf("expected ErrCompleted, got %v", err)
	}
}
