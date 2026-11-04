package get_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/EgorTrytnev/TripGo/internal/model"
	gettrip "github.com/EgorTrytnev/TripGo/internal/usecase/trip/get"
)

func TestGetTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	id := uuid.New()
	expected := model.Trip{ID: id}
	repo.EXPECT().GetTripByID(gomock.Any(), id).Return(expected, nil)
	got, err := gettrip.New(repo).Execute(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("got %s, want %s", got.ID, id)
	}
}
