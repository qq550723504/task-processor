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

	"task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

func TestProductEnrichTaskRepositoryTenantScopeContract(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) productenrich.TaskRepository
	}{
		{name: "gorm", new: newProductEnrichGORMRepository},
		{name: "memory", new: func(*testing.T) productenrich.TaskRepository { return NewMemTaskRepository() }},
	}

	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			runProductEnrichTenantScopeContract(t, factory.new)
		})
	}
}

func runProductEnrichTenantScopeContract(t *testing.T, newRepository func(*testing.T) productenrich.TaskRepository) {
	t.Helper()
	unscoped := context.Background()
	tenantA := aiidentity.WithIdentity(unscoped, aiidentity.Identity{TenantID: " tenant-a "})

	t.Run("GetTask", func(t *testing.T) {
		repo := seedProductEnrichTenantTasks(t, newRepository)
		own, err := repo.GetTask(tenantA, "task-a")
		require.NoError(t, err)
		require.Equal(t, "task-a", own.ID)

		_, err = repo.GetTask(tenantA, "task-b")
		require.ErrorIs(t, err, productenrich.ErrTaskNotFound)
		_, err = repo.GetTask(unscoped, "task-b")
		require.NoError(t, err)
	})

	mutations := []struct {
		name string
		run  func(productenrich.TaskRepository) error
	}{
		{name: "MarkProcessing", run: func(repo productenrich.TaskRepository) error { return repo.MarkProcessing(tenantA, "task-b") }},
		{name: "MarkCompleted", run: func(repo productenrich.TaskRepository) error {
			return repo.MarkCompleted(tenantA, "task-b", &productenrich.ProductJSON{Title: "forbidden"})
		}},
		{name: "MarkFailed", run: func(repo productenrich.TaskRepository) error { return repo.MarkFailed(tenantA, "task-b", "forbidden") }},
		{name: "PrepareRetry", run: func(repo productenrich.TaskRepository) error { return repo.PrepareRetry(tenantA, "task-b") }},
		{name: "UpdateTaskStatus", run: func(repo productenrich.TaskRepository) error {
			return repo.UpdateTaskStatus(tenantA, "task-b", productenrich.TaskStatusCompleted)
		}},
		{name: "UpdateTaskError", run: func(repo productenrich.TaskRepository) error {
			return repo.UpdateTaskError(tenantA, "task-b", "forbidden")
		}},
		{name: "SaveTaskResult", run: func(repo productenrich.TaskRepository) error {
			return repo.SaveTaskResult(tenantA, "task-b", &productenrich.ProductJSON{Title: "forbidden"})
		}},
		{name: "IncrementRetryCount", run: func(repo productenrich.TaskRepository) error { return repo.IncrementRetryCount(tenantA, "task-b") }},
		{name: "ResetForRetry", run: func(repo productenrich.TaskRepository) error { return repo.ResetForRetry(tenantA, "task-b") }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			repo := seedProductEnrichTenantTasks(t, newRepository)
			before, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)

			err = mutation.run(repo)
			require.ErrorIs(t, err, productenrich.ErrTaskNotFound)

			after, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)
			require.Equal(t, before, after, "cross-tenant mutation changed task-b")
		})
	}
}

func seedProductEnrichTenantTasks(t *testing.T, newRepository func(*testing.T) productenrich.TaskRepository) productenrich.TaskRepository {
	t.Helper()
	repo := newRepository(t)
	for _, task := range []*productenrich.Task{
		{
			ID: "task-a", Status: productenrich.TaskStatusPending, TenantID: "tenant-b", UserID: "legacy-user-b",
			PersistedExecutionEnvelope: productEnrichExecutionEnvelope("tenant-a"),
		},
		{
			ID: "task-b", Status: productenrich.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user-a",
			PersistedExecutionEnvelope: productEnrichExecutionEnvelope("tenant-b"),
		},
	} {
		require.NoError(t, repo.CreateTask(context.Background(), task))
	}
	return repo
}

func productEnrichExecutionEnvelope(tenantID string) aiidentity.PersistedExecutionEnvelope {
	return aiidentity.PersistedExecutionEnvelope{
		ExecutionIdentityVersion: 1,
		ExecutionTenantID:        tenantID,
		ExecutionUserID:          "user-" + tenantID,
		ExecutionSourcePlatform:  "productenrich",
		ExecutionSourceTaskType:  "product",
	}
}

func newProductEnrichGORMRepository(t *testing.T) productenrich.TaskRepository {
	t.Helper()
	registerProductEnrichTenantScopeSQLite.Do(func() {
		sql.Register("productenrich_tenant_scope_sqlite", &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("NOW", func() string { return "2026-08-22 00:00:00" }, true)
		}})
	})
	db, err := gorm.Open(sqlite.Dialector{DriverName: "productenrich_tenant_scope_sqlite", DSN: ":memory:"}, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&productenrich.Task{}))
	return NewTaskRepository(db)
}

var registerProductEnrichTenantScopeSQLite sync.Once
