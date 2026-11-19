//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/EgorTrytnev/TripGo/internal/domain/trip"
	"github.com/EgorTrytnev/TripGo/internal/infra/postgres"
	"github.com/EgorTrytnev/TripGo/internal/model"
	"github.com/EgorTrytnev/TripGo/internal/repository"
)

func TestCreateTripAndHistoryRollbackTogether(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL must point at a migrated PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := repository.NewTripRepository(pool, 3*time.Second)
	tx := postgres.NewTxManager(pool, 3*time.Second)
	id := uuid.New()
	driver := uuid.New()
	now := time.Now().UTC()
	trip := model.Trip{ID: id, UserID: uuid.New(), DriverID: driver, StartLat: 59.9, StartLong: 30.3, EndLat: 59.8, EndLong: 30.4, Price: 100, Status: "active", StartedAt: now, CreatedAt: now, UpdatedAt: now}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id=$1", id) })
	err = tx.Do(ctx, func(txCtx context.Context) error {
		if err := repo.CreateTrip(txCtx, trip); err != nil {
			return err
		}
		return repo.CreateHistory(txCtx, model.TripStatusHistory{TripID: id, ToStatus: "not-a-status", Reason: "intentional constraint violation", ChangedAt: now})
	})
	if err == nil {
		t.Fatal("expected history insert to fail")
	}
	if _, err := repo.GetTripByID(ctx, id); err == nil {
		t.Fatal("trip row survived transaction rollback")
	}
}

func TestCreateTripRejectsBusyDriver(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL must point at a migrated PostgreSQL database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := repository.NewTripRepository(pool, 3*time.Second)
	now := time.Now().UTC()
	item := model.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusActive, StartedAt: now, CreatedAt: now, UpdatedAt: now}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM trips WHERE driver_id=$1", item.DriverID) }()
	if err := repo.CreateTrip(ctx, item); err != nil {
		t.Fatal(err)
	}
	item.ID = uuid.New()
	if err := repo.CreateTrip(ctx, item); !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("got %v, want ErrDriverBusy", err)
	}
}

func TestFindStaleTripsSkipsFreshAndCompleted(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := repository.NewTripRepository(pool, 3*time.Second)
	now := time.Now().UTC()
	oldID := uuid.New()
	freshID := uuid.New()
	completedID := uuid.New()
	old := model.Trip{ID: oldID, UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusActive, StartedAt: now.Add(-10 * time.Minute), CreatedAt: now, UpdatedAt: now}
	fresh := model.Trip{ID: freshID, UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusActive, StartedAt: now, CreatedAt: now, UpdatedAt: now}
	finished := now.Add(-time.Minute)
	completed := model.Trip{ID: completedID, UserID: uuid.New(), DriverID: uuid.New(), Status: domaintrip.StatusCompleted, StartedAt: now.Add(-10 * time.Minute), FinishedAt: &finished, CreatedAt: now, UpdatedAt: now}
	for _, item := range []model.Trip{old, fresh, completed} {
		if err := repo.CreateTrip(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = ANY($1)", []uuid.UUID{oldID, freshID, completedID})
	}()
	if err := repo.UpdateLastPositionAt(ctx, freshID, now); err != nil {
		t.Fatal(err)
	}
	items, err := repo.FindStaleTrips(ctx, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != oldID {
		t.Fatalf("got %+v, want only %s", items, oldID)
	}
}
