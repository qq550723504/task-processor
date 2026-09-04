//go:build integration

package storecenter_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"task-processor/internal/storecenter"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostgresNoHistoryBackfillConcurrentWorkersUpdateEachRowOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("storehistory"),
		tcpostgres.WithUsername("storehistory"),
		tcpostgres.WithPassword("storehistory"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}

	const rows = 12
	for index := 1; index <= rows; index++ {
		seedLegacyStoreRow(t, db, fmt.Sprintf("00000000-0000-4000-8000-%012d", index), "org-postgres", "active", 2, nil)
	}
	now := time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC)
	workers := []*storecenter.GormStoreHistoryMigrator{
		newNoHistoryMigrator(t, db, now),
		newNoHistoryMigrator(t, db, now),
	}

	reports := make(chan storecenter.StoreHistoryMigrationReport, len(workers))
	errorsFound := make(chan error, len(workers))
	var wait sync.WaitGroup
	for _, worker := range workers {
		wait.Add(1)
		go func(migrator *storecenter.GormStoreHistoryMigrator) {
			defer wait.Done()
			report, runErr := migrator.BackfillBatch(ctx, rows)
			reports <- report
			errorsFound <- runErr
		}(worker)
	}
	wait.Wait()
	close(reports)
	close(errorsFound)
	for runErr := range errorsFound {
		if runErr != nil {
			t.Fatalf("concurrent backfill: %v", runErr)
		}
	}
	var updated int64
	for report := range reports {
		updated += report.UpdatedCount
	}
	if updated != rows {
		t.Fatalf("combined updated rows = %d, want %d", updated, rows)
	}

	verification, err := workers[0].Verify(ctx)
	if err != nil || !verification.ReadyForConstraints || verification.HistoryConfirmedAbsentCount != rows {
		t.Fatalf("Verify() = %+v, %v", verification, err)
	}
	var versionSum int64
	if err := db.Table("workbench_stores").Select("COALESCE(SUM(version), 0)").Scan(&versionSum).Error; err != nil {
		t.Fatal(err)
	}
	if versionSum != rows*3 {
		t.Fatalf("version sum = %d, want %d", versionSum, rows*3)
	}
}
