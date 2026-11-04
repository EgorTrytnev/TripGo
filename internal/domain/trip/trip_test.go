package trip

import (
	"testing"
	"time"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

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
