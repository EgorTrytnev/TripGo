// Пакет positions выполняет фоновый опрос устаревших поездок.
package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/EgorTrytnev/TripGo/internal/model"
)

type tripScanner interface {
	FindStaleTrips(context.Context, time.Duration) ([]model.StaleTrip, error)
}

type pushSender interface {
	SendPositionRequest(context.Context, uuid.UUID, uuid.UUID) error
}

type job struct {
	trip model.StaleTrip
	span trace.SpanContext
}

// Metrics хранит метрики опроса и пула.
type Metrics struct {
	Checks   metric.Int64Counter
	Queue    metric.Int64UpDownCounter
	Active   metric.Int64UpDownCounter
	Requests metric.Int64Counter
	Duration metric.Float64Histogram
}

// NewMetrics создаёт метрики опроса координат.
func NewMetrics(m metric.Meter) (*Metrics, error) {
	checks, err := m.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, err
	}
	queue, err := m.Int64UpDownCounter("tripgo.position_requests.queue.size")
	if err != nil {
		return nil, err
	}
	active, err := m.Int64UpDownCounter("tripgo.position_requests.workers.active")
	if err != nil {
		return nil, err
	}
	requests, err := m.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, err
	}
	duration, err := m.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	return &Metrics{checks, queue, active, requests, duration}, nil
}

// Worker объединяет сканер, очередь и воркеры.
type Worker struct {
	repo                          tripScanner
	push                          pushSender
	logger                        *slog.Logger
	interval, staleAfter, timeout time.Duration
	workers                       int
	queue                         chan *job
	metrics                       *Metrics
	mu                            sync.Mutex
	pending                       map[uuid.UUID]*job
	canceled                      map[uuid.UUID]struct{}
	scanning                      bool
	stopScan                      context.CancelFunc
	stopWorkers                   context.CancelFunc
	scanDone                      chan struct{}
	done                          chan struct{}
	newTicker                     func(time.Duration) ticker
}

type ticker interface {
	Ticks() <-chan time.Time
	Stop()
}

type realTicker struct{ *time.Ticker }

func (t realTicker) Ticks() <-chan time.Time { return t.C }

// New создаёт компонент опроса.
func New(repo tripScanner, sender pushSender, interval, staleAfter, timeout time.Duration, workers, queueSize int, logger *slog.Logger, metrics *Metrics) *Worker {
	return &Worker{repo: repo, push: sender, interval: interval, staleAfter: staleAfter, timeout: timeout, workers: workers, queue: make(chan *job, queueSize), logger: logger, metrics: metrics, pending: make(map[uuid.UUID]*job), canceled: make(map[uuid.UUID]struct{}), scanDone: make(chan struct{}), done: make(chan struct{}), newTicker: func(d time.Duration) ticker { return realTicker{time.NewTicker(d)} }}
}

// Start запускает сканер и заданное число воркеров.
func (w *Worker) Start(ctx context.Context) {
	scanCtx, stopScan := context.WithCancel(ctx)
	workerCtx, stopWorkers := context.WithCancel(context.WithoutCancel(ctx))
	w.stopScan, w.stopWorkers = stopScan, stopWorkers
	var group sync.WaitGroup
	group.Add(w.workers)
	for i := 0; i < w.workers; i++ {
		go func() { defer group.Done(); w.work(workerCtx) }()
	}
	go func() {
		defer close(w.scanDone)
		w.scan(scanCtx)
		close(w.queue)
	}()
	go func() { group.Wait(); close(w.done) }()
}

// Stop останавливает сканер и даёт воркерам обработать очередь.
func (w *Worker) Stop(ctx context.Context) error {
	w.stopScan()
	select {
	case <-w.scanDone:
		select {
		case <-w.done:
			w.stopWorkers()
			return nil
		case <-ctx.Done():
			w.stopWorkers()
			<-w.done
			return ctx.Err()
		}
	case <-ctx.Done():
		w.stopWorkers()
		<-w.scanDone
		<-w.done
		return ctx.Err()
	}
}

// Cancel отменяет ожидающую задачу поездки.
func (w *Worker) Cancel(tripID uuid.UUID) {
	w.mu.Lock()
	delete(w.pending, tripID)
	if w.scanning {
		w.canceled[tripID] = struct{}{}
	}
	w.mu.Unlock()
}

func (w *Worker) scan(ctx context.Context) {
	ticker := w.newTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Ticks():
			w.scanOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) scanOnce(ctx context.Context) {
	w.mu.Lock()
	w.scanning = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.scanning = false
		clear(w.canceled)
		w.mu.Unlock()
	}()
	ctx, span := otel.Tracer("tripgo.positions").Start(ctx, "scan stale trips")
	defer span.End()
	trips, err := w.repo.FindStaleTrips(ctx, w.staleAfter)
	if err != nil {
		if ctx.Err() == nil && w.logger != nil {
			w.logger.ErrorContext(ctx, "stale trips scan failed", "error", err)
		}
		return
	}
	for _, trip := range trips {
		j := &job{trip: trip, span: span.SpanContext()}
		w.mu.Lock()
		result := "duplicate"
		_, canceled := w.canceled[trip.ID]
		if _, exists := w.pending[trip.ID]; !canceled && !exists {
			select {
			case w.queue <- j:
				w.pending[trip.ID] = j
				result = "queued"
				if w.metrics != nil {
					w.metrics.Queue.Add(ctx, 1)
				}
			default:
				result = "dropped"
			}
		}
		w.mu.Unlock()
		if w.metrics != nil {
			w.metrics.Checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
		}
		if result == "dropped" && w.logger != nil {
			w.logger.WarnContext(ctx, "position request queue is full", "trip_id", trip.ID)
		}
	}
}

func (w *Worker) work(ctx context.Context) {
	for {
		var j *job
		var ok bool
		select {
		case j, ok = <-w.queue:
			if !ok {
				return
			}
		case <-ctx.Done():
			return
		}
		if w.metrics != nil {
			w.metrics.Queue.Add(ctx, -1)
		}
		w.mu.Lock()
		valid := w.pending[j.trip.ID] == j
		w.mu.Unlock()
		if !valid {
			continue
		}
		jobCtx, span := otel.Tracer("tripgo.positions").Start(ctx, "position request", trace.WithLinks(trace.Link{SpanContext: j.span}))
		jobCtx, cancel := context.WithTimeout(jobCtx, w.timeout)
		if w.metrics != nil {
			w.metrics.Active.Add(jobCtx, 1)
		}
		start := time.Now()
		err := w.push.SendPositionRequest(jobCtx, j.trip.DriverID, j.trip.ID)
		result := "success"
		if err != nil {
			result = "error"
			if w.logger != nil {
				w.logger.ErrorContext(jobCtx, "position request failed", "error", err)
			}
		}
		if w.metrics != nil {
			w.metrics.Requests.Add(jobCtx, 1, metric.WithAttributes(attribute.String("result", result)))
			w.metrics.Duration.Record(jobCtx, time.Since(start).Seconds())
			w.metrics.Active.Add(jobCtx, -1)
		}
		cancel()
		span.End()
		w.mu.Lock()
		if w.pending[j.trip.ID] == j {
			delete(w.pending, j.trip.ID)
		}
		w.mu.Unlock()
	}
}
