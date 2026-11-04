package create_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/model"
	"github.com/EgorTrytnev/TripGo/internal/usecase/trip/create"
)

func TestCreateTrip(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	counter := NewMocktripCounter(ctrl)
	u := create.New(repo, tx, counter, slog.New(slog.NewTextHandler(io.Discard, nil)))
	input := create.Input{UserID: uuid.New(), DriverID: uuid.New(), Price: 1450}
	repo.EXPECT().CreateTrip(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, item model.Trip) error {
		if item.Status != trip.StatusActive || item.UserID != input.UserID || item.Price != input.Price {
			return errors.New("trip fields were not mapped")
		}
		return nil
	})
	repo.EXPECT().CreateHistory(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, h model.TripStatusHistory) error {
		if h.ToStatus != trip.StatusActive || h.TripID == uuid.Nil {
			return errors.New("history fields were not mapped")
		}
		return nil
	})
	tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(c context.Context, fn func(context.Context) error) error { return fn(c) })
	counter.EXPECT().AddCreated(ctx, int64(1))
	result, err := u.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID == uuid.Nil || result.Status != trip.StatusActive {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCreateTripRollsBackOnRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	counter := NewMocktripCounter(ctrl)
	failure := errors.New("database down")
	repo.EXPECT().CreateTrip(gomock.Any(), gomock.Any()).Return(failure)
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(c context.Context, fn func(context.Context) error) error { return fn(c) })
	result, err := create.New(repo, tx, counter, slog.New(slog.NewTextHandler(io.Discard, nil))).Execute(context.Background(), create.Input{})
	if !errors.Is(err, failure) {
		t.Fatalf("expected repository error, got %v", err)
	}
	if result.ID != uuid.Nil {
		t.Fatalf("expected empty result after failure, got %+v", result)
	}
}
