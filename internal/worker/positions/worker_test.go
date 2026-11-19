package positions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

type scannerStub struct {
	trips []model.StaleTrip
}

func (s scannerStub) FindStaleTrips(context.Context, time.Duration) ([]model.StaleTrip, error) {
	return s.trips, nil
}

type senderStub struct {
	mu    sync.Mutex
	ids   []uuid.UUID
	err   error
	block chan struct{}
}

func (s *senderStub) SendPositionRequest(ctx context.Context, _, tripID uuid.UUID) error {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	s.ids = append(s.ids, tripID)
	s.mu.Unlock()
	return s.err
}

func (s *senderStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ids)
}

func staleTrip() model.StaleTrip {
	return model.StaleTrip{ID: uuid.New(), DriverID: uuid.New(), LastPositionAt: time.Now()}
}

func TestWorkerDeduplicatesTrips(t *testing.T) {
	trip := staleTrip()
	sender := &senderStub{block: make(chan struct{})}
	w := New(scannerStub{trips: []model.StaleTrip{trip}}, sender, time.Hour, time.Minute, time.Second, 1, 2, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	w.scanOnce(ctx)
	w.scanOnce(ctx)
	w.mu.Lock()
	count := len(w.pending)
	w.mu.Unlock()
	if count != 1 {
		t.Fatalf("pending=%d, want 1", count)
	}
	close(sender.block)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	cancel()
}

func TestWorkerCancelPreventsPush(t *testing.T) {
	trip := staleTrip()
	sender := &senderStub{}
	w := New(scannerStub{trips: []model.StaleTrip{trip}}, sender, time.Hour, time.Minute, time.Second, 1, 1, nil, nil)
	ctx := context.Background()
	w.scanOnce(ctx)
	w.Cancel(trip.ID)
	w.Start(ctx)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 0 {
		t.Fatalf("pushes=%d, want 0", sender.count())
	}
}

func TestWorkerDropsWhenQueueIsFull(t *testing.T) {
	trips := []model.StaleTrip{staleTrip(), staleTrip()}
	sender := &senderStub{block: make(chan struct{})}
	w := New(scannerStub{trips: trips}, sender, time.Hour, time.Minute, time.Second, 1, 1, nil, nil)
	ctx := context.Background()
	w.scanOnce(ctx)
	w.mu.Lock()
	count := len(w.pending)
	w.mu.Unlock()
	if count != 1 {
		t.Fatalf("pending=%d, want 1", count)
	}
	close(sender.block)
	w.Start(ctx)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerContinuesAfterPushError(t *testing.T) {
	first, second := staleTrip(), staleTrip()
	sender := &senderStub{err: errors.New("push failed")}
	w := New(scannerStub{trips: []model.StaleTrip{first, second}}, sender, time.Hour, time.Minute, time.Second, 1, 2, nil, nil)
	ctx := context.Background()
	w.Start(ctx)
	w.scanOnce(ctx)
	if err := w.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.count() != 2 {
		t.Fatalf("pushes=%d, want 2", sender.count())
	}
}
