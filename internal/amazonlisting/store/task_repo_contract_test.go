package store

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/shared/aiidentity"
)

func TestAmazonTaskRepositoryTenantScopeContract(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) amazonlisting.Repository
	}{
		{name: "gorm", new: newAmazonGORMRepository},
		{name: "memory", new: func(*testing.T) amazonlisting.Repository { return NewMemTaskRepository() }},
	}

	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			runAmazonTenantScopeContract(t, factory.new)
		})
	}
}

func runAmazonTenantScopeContract(t *testing.T, newRepository func(*testing.T) amazonlisting.Repository) {
	t.Helper()
	unscoped := context.Background()
	tenantA := aiidentity.WithIdentity(unscoped, aiidentity.Identity{TenantID: " tenant-a "})

	t.Run("GetTask", func(t *testing.T) {
		repo := seedAmazonTenantTasks(t, newRepository)
		own, err := repo.GetTask(tenantA, "task-a")
		require.NoError(t, err)
		require.Equal(t, "task-a", own.ID)

		_, err = repo.GetTask(tenantA, "task-b")
		require.ErrorIs(t, err, amazonlisting.ErrTaskNotFound)
		_, err = repo.GetTask(unscoped, "task-b")
		require.NoError(t, err)
	})

	t.Run("ListTasks", func(t *testing.T) {
		repo := seedAmazonTenantTasks(t, newRepository)
		items, err := repo.ListTasks(tenantA, nil, 0)
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, "task-a", items[0].ID)

		items, err = repo.ListTasks(unscoped, nil, 0)
		require.NoError(t, err)
		require.Len(t, items, 2)
	})

	t.Run("normalizes persisted execution tenant", func(t *testing.T) {
		t.Run("GetTask", func(t *testing.T) {
			repo := seedAmazonSpacedTenantTask(t, newRepository)
			task, err := repo.GetTask(tenantA, "task-spaced")
			require.NoError(t, err)
			require.Equal(t, "task-spaced", task.ID)
		})

		t.Run("ListTasks", func(t *testing.T) {
			repo := seedAmazonSpacedTenantTask(t, newRepository)
			items, err := repo.ListTasks(tenantA, nil, 0)
			require.NoError(t, err)
			require.Len(t, items, 1)
			require.Equal(t, "task-spaced", items[0].ID)
		})

		t.Run("mutation", func(t *testing.T) {
			repo := seedAmazonSpacedTenantTask(t, newRepository)
			require.NoError(t, repo.IncrementRetryCount(tenantA, "task-spaced"))
			task, err := repo.GetTask(unscoped, "task-spaced")
			require.NoError(t, err)
			require.Equal(t, 1, task.RetryCount)
		})
	})

	mutations := []struct {
		name string
		run  func(amazonlisting.Repository) error
	}{
		{name: "MarkProcessing", run: func(repo amazonlisting.Repository) error { return repo.MarkProcessing(tenantA, "task-b") }},
		{name: "MarkCompleted", run: func(repo amazonlisting.Repository) error {
			return repo.MarkCompleted(tenantA, "task-b", &amazonlisting.AmazonListingDraft{TaskID: "forbidden"})
		}},
		{name: "MarkNeedsReview", run: func(repo amazonlisting.Repository) error {
			return repo.MarkNeedsReview(tenantA, "task-b", &amazonlisting.AmazonListingDraft{TaskID: "forbidden"}, "forbidden")
		}},
		{name: "MarkRejected", run: func(repo amazonlisting.Repository) error { return repo.MarkRejected(tenantA, "task-b", "forbidden") }},
		{name: "MarkFailed", run: func(repo amazonlisting.Repository) error { return repo.MarkFailed(tenantA, "task-b", "forbidden") }},
		{name: "PrepareRetry", run: func(repo amazonlisting.Repository) error { return repo.PrepareRetry(tenantA, "task-b") }},
		{name: "IncrementRetryCount", run: func(repo amazonlisting.Repository) error { return repo.IncrementRetryCount(tenantA, "task-b") }},
		{name: "UpdateTaskStatus", run: func(repo amazonlisting.Repository) error {
			return repo.UpdateTaskStatus(tenantA, "task-b", amazonlisting.TaskStatusCompleted)
		}},
		{name: "UpdateTaskError", run: func(repo amazonlisting.Repository) error { return repo.UpdateTaskError(tenantA, "task-b", "forbidden") }},
		{name: "SaveTaskResult", run: func(repo amazonlisting.Repository) error {
			return repo.SaveTaskResult(tenantA, "task-b", &amazonlisting.AmazonListingDraft{TaskID: "forbidden"})
		}},
		{name: "ResetForRetry", run: func(repo amazonlisting.Repository) error { return repo.ResetForRetry(tenantA, "task-b") }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			repo := seedAmazonTenantTasks(t, newRepository)
			before, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)

			err = mutation.run(repo)
			require.ErrorIs(t, err, amazonlisting.ErrTaskNotFound)

			after, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)
			require.Equal(t, before, after, "cross-tenant mutation changed task-b")
		})
	}
}

func seedAmazonTenantTasks(t *testing.T, newRepository func(*testing.T) amazonlisting.Repository) amazonlisting.Repository {
	t.Helper()
	repo := newRepository(t)
	for _, task := range []*amazonlisting.Task{
		{ID: "task-a", Status: amazonlisting.TaskStatusPending, PersistedExecutionEnvelope: amazonExecutionEnvelope("tenant-a")},
		{ID: "task-b", Status: amazonlisting.TaskStatusPending, PersistedExecutionEnvelope: amazonExecutionEnvelope("tenant-b")},
	} {
		require.NoError(t, repo.CreateTask(context.Background(), task))
	}
	return repo
}

func seedAmazonSpacedTenantTask(t *testing.T, newRepository func(*testing.T) amazonlisting.Repository) amazonlisting.Repository {
	t.Helper()
	repo := newRepository(t)
	require.NoError(t, repo.CreateTask(context.Background(), &amazonlisting.Task{
		ID:                         "task-spaced",
		Status:                     amazonlisting.TaskStatusPending,
		PersistedExecutionEnvelope: amazonExecutionEnvelope(" tenant-a "),
	}))
	return repo
}

func amazonExecutionEnvelope(tenantID string) aiidentity.PersistedExecutionEnvelope {
	return aiidentity.PersistedExecutionEnvelope{
		ExecutionIdentityVersion: 1,
		ExecutionTenantID:        tenantID,
		ExecutionUserID:          "user-" + tenantID,
		ExecutionSourcePlatform:  "amazon",
		ExecutionSourceTaskType:  "listing",
	}
}

func newAmazonGORMRepository(t *testing.T) amazonlisting.Repository {
	t.Helper()
	const driverName = "amazon_tenant_scope_sqlite"
	registerAmazonTenantScopeSQLite.Do(func() {
		sql.Register(driverName, &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("NOW", func() string { return "2026-08-22 00:00:00" }, true)
		}})
	})
	db, err := gorm.Open(sqlite.Dialector{DriverName: driverName, DSN: ":memory:"}, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&amazonlisting.Task{}))
	return NewTaskRepository(db)
}

var registerAmazonTenantScopeSQLite sync.Once
