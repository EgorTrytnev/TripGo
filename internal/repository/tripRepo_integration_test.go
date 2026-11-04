//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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
