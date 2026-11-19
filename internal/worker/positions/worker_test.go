package positions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

type fakeTicker struct{ ticks chan time.Time }

func (t *fakeTicker) Ticks() <-chan time.Time { return t.ticks }
func (t *fakeTicker) Stop()                   {}

type scannerStub struct {
	trips []model.StaleTrip
}

type blockingScanner struct {
	started chan struct{}
	release chan struct{}
	trip    model.StaleTrip
}

func (s *blockingScanner) FindStaleTrips(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
	close(s.started)
	select {
	case <-s.release:
		return []model.StaleTrip{s.trip}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestCancelDuringScan(t *testing.T) {
	trip := staleTrip()
	scanner := &blockingScanner{started: make(chan struct{}), release: make(chan struct{}), trip: trip}
	w := New(scanner, &senderStub{}, time.Hour, time.Minute, time.Second, 1, 1, nil, nil)
	done := make(chan struct{})
	go func() { w.scanOnce(context.Background()); close(done) }()
	<-scanner.started
	w.Cancel(trip.ID)
	close(scanner.release)
	<-done
	if len(w.queue) != 0 {
		t.Fatal("отменённая задача попала в очередь")
	}
}

func TestStopCancelsActiveScan(t *testing.T) {
	scanner := &blockingScanner{started: make(chan struct{}), release: make(chan struct{})}
	w := New(scanner, &senderStub{}, time.Hour, time.Minute, time.Second, 1, 1, nil, nil)
	tick := &fakeTicker{ticks: make(chan time.Time, 1)}
	w.newTicker = func(time.Duration) ticker { return tick }
	w.Start(context.Background())
	tick.ticks <- time.Now()
	<-scanner.started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.scanDone:
	default:
		t.Fatal("сканер не завершился")
	}
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
