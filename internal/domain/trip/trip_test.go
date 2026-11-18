package trip

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

func TestValidate(t *testing.T) {
	id := uuid.New()
	err := Validate(id, id, 91, 0, 0, 0, 1)
	var validation ValidationError
	if !errors.As(err, &validation) || validation.Field != "start_point.latitude" {
		t.Fatalf("got %v, want start point validation error", err)
	}
}

func TestFinish(t *testing.T) {
	at := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	t.Run("active trip completes", func(t *testing.T) {
		item := model.Trip{Status: StatusActive}
		if err := Finish(&item, at); err != nil {
			t.Fatal(err)
		}
		if item.Status != StatusCompleted || item.FinishedAt == nil || !item.FinishedAt.Equal(at) || !item.UpdatedAt.Equal(at) {
			t.Fatalf("unexpected completed trip: %+v", item)
		}
	})
	t.Run("completed trip stays completed", func(t *testing.T) {
		item := model.Trip{Status: StatusCompleted}
		if err := Finish(&item, at); err != ErrCompleted {
			t.Fatalf("expected ErrCompleted, got %v", err)
		}
		if item.FinishedAt != nil {
			t.Fatal("invalid transition mutated the trip")
		}
	})
}
