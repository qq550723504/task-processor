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

	"task-processor/internal/productimage"
	"task-processor/internal/shared/aiidentity"
)

func TestProductImageTaskRepositoryTenantScopeContract(t *testing.T) {
	factories := []struct {
		name string
		new  func(*testing.T) productimage.TaskRepository
	}{
		{name: "gorm", new: newProductImageGORMRepository},
		{name: "memory", new: func(*testing.T) productimage.TaskRepository { return NewMemTaskRepository() }},
	}

	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			runProductImageTenantScopeContract(t, factory.new)
		})
	}
}

func runProductImageTenantScopeContract(t *testing.T, newRepository func(*testing.T) productimage.TaskRepository) {
	t.Helper()
	unscoped := context.Background()
	tenantA := aiidentity.WithIdentity(unscoped, aiidentity.Identity{TenantID: " tenant-a "})

	t.Run("CreateTask", func(t *testing.T) {
		t.Run("matching scoped envelope", func(t *testing.T) {
			repo := newRepository(t)
			task := &productimage.Task{ID: "create-own", Status: productimage.TaskStatusPending, PersistedExecutionEnvelope: productImageExecutionEnvelope(" tenant-a ")}
			require.NoError(t, repo.CreateTask(tenantA, task))
			created, err := repo.GetTask(unscoped, task.ID)
			require.NoError(t, err)
			require.Equal(t, task.ID, created.ID)
		})

		for _, tc := range []struct {
			name     string
			envelope aiidentity.PersistedExecutionEnvelope
			legacyID string
		}{
			{name: "cross tenant", envelope: productImageExecutionEnvelope("tenant-b"), legacyID: "tenant-a"},
			{name: "absent execution envelope", legacyID: "tenant-a"},
			{name: "partial execution envelope", envelope: aiidentity.PersistedExecutionEnvelope{ExecutionTenantID: "tenant-a"}, legacyID: "tenant-a"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo := newRepository(t)
				task := &productimage.Task{ID: "create-rejected", TenantID: tc.legacyID, Status: productimage.TaskStatusPending, PersistedExecutionEnvelope: tc.envelope}
				require.ErrorIs(t, repo.CreateTask(tenantA, task), productimage.ErrTaskNotFound)
				_, err := repo.GetTask(unscoped, task.ID)
				require.ErrorIs(t, err, productimage.ErrTaskNotFound, "rejected create persisted a row")
			})
		}
	})

	t.Run("GetTask", func(t *testing.T) {
		repo := seedProductImageTenantTasks(t, newRepository)
		own, err := repo.GetTask(tenantA, "task-a")
		require.NoError(t, err)
		require.Equal(t, "task-a", own.ID)

		_, err = repo.GetTask(tenantA, "task-b")
		require.ErrorIs(t, err, productimage.ErrTaskNotFound)
		_, err = repo.GetTask(unscoped, "task-b")
		require.NoError(t, err)
	})

	t.Run("legacy owner does not authorize an absent execution envelope", func(t *testing.T) {
		repo := newRepository(t)
		require.NoError(t, repo.CreateTask(unscoped, &productimage.Task{
			ID: "legacy-task", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user-a",
		}))

		_, err := repo.GetTask(tenantA, "legacy-task")
		require.ErrorIs(t, err, productimage.ErrTaskNotFound)
		err = repo.IncrementRetryCount(tenantA, "legacy-task")
		require.ErrorIs(t, err, productimage.ErrTaskNotFound)
		legacy, err := repo.GetTask(unscoped, "legacy-task")
		require.NoError(t, err)
		require.Equal(t, 0, legacy.RetryCount)
	})

	t.Run("normalizes persisted execution tenant", func(t *testing.T) {
		t.Run("GetTask", func(t *testing.T) {
			repo := seedProductImageSpacedTenantTask(t, newRepository)
			task, err := repo.GetTask(tenantA, "task-spaced")
			require.NoError(t, err)
			require.Equal(t, "task-spaced", task.ID)
		})

		t.Run("mutation", func(t *testing.T) {
			repo := seedProductImageSpacedTenantTask(t, newRepository)
			require.NoError(t, repo.IncrementRetryCount(tenantA, "task-spaced"))
			task, err := repo.GetTask(unscoped, "task-spaced")
			require.NoError(t, err)
			require.Equal(t, 1, task.RetryCount)
		})
	})

	for _, noncanonical := range []struct {
		name              string
		persistedTenantID string
	}{
		{name: "tab padding", persistedTenantID: "\ttenant-a\t"},
		{name: "Unicode space padding", persistedTenantID: "\u00a0tenant-a\u00a0"},
	} {
		t.Run("rejects noncanonical persisted tenant/"+noncanonical.name, func(t *testing.T) {
			t.Run("GetTask", func(t *testing.T) {
				repo := seedProductImageNoncanonicalTenantTask(t, newRepository, noncanonical.persistedTenantID)
				_, err := repo.GetTask(tenantA, "task-noncanonical")
				require.ErrorIs(t, err, productimage.ErrTaskNotFound)
			})

			t.Run("mutation", func(t *testing.T) {
				repo := seedProductImageNoncanonicalTenantTask(t, newRepository, noncanonical.persistedTenantID)
				before, err := repo.GetTask(unscoped, "task-noncanonical")
				require.NoError(t, err)

				err = repo.IncrementRetryCount(tenantA, "task-noncanonical")
				require.ErrorIs(t, err, productimage.ErrTaskNotFound)

				after, err := repo.GetTask(unscoped, "task-noncanonical")
				require.NoError(t, err)
				require.Equal(t, before, after, "noncanonical tenant mutation changed task")
			})
		})
	}

	mutations := []struct {
		name string
		run  func(productimage.TaskRepository) error
	}{
		{name: "MarkProcessing", run: func(repo productimage.TaskRepository) error { return repo.MarkProcessing(tenantA, "task-b") }},
		{name: "MarkCompleted", run: func(repo productimage.TaskRepository) error {
			return repo.MarkCompleted(tenantA, "task-b", &productimage.ImageProcessResult{})
		}},
		{name: "MarkNeedsReview", run: func(repo productimage.TaskRepository) error {
			return repo.MarkNeedsReview(tenantA, "task-b", &productimage.ImageProcessResult{}, "forbidden")
		}},
		{name: "MarkRejected", run: func(repo productimage.TaskRepository) error { return repo.MarkRejected(tenantA, "task-b", "forbidden") }},
		{name: "MarkFailed", run: func(repo productimage.TaskRepository) error { return repo.MarkFailed(tenantA, "task-b", "forbidden") }},
		{name: "PrepareRetry", run: func(repo productimage.TaskRepository) error { return repo.PrepareRetry(tenantA, "task-b") }},
		{name: "UpdateTaskStatus", run: func(repo productimage.TaskRepository) error {
			return repo.UpdateTaskStatus(tenantA, "task-b", productimage.TaskStatusCompleted)
		}},
		{name: "UpdateTaskError", run: func(repo productimage.TaskRepository) error {
			return repo.UpdateTaskError(tenantA, "task-b", "forbidden")
		}},
		{name: "SaveTaskResult", run: func(repo productimage.TaskRepository) error {
			return repo.SaveTaskResult(tenantA, "task-b", &productimage.ImageProcessResult{})
		}},
		{name: "IncrementRetryCount", run: func(repo productimage.TaskRepository) error { return repo.IncrementRetryCount(tenantA, "task-b") }},
		{name: "ResetForRetry", run: func(repo productimage.TaskRepository) error { return repo.ResetForRetry(tenantA, "task-b") }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			repo := seedProductImageTenantTasks(t, newRepository)
			before, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)

			err = mutation.run(repo)
			require.ErrorIs(t, err, productimage.ErrTaskNotFound)

			after, err := repo.GetTask(unscoped, "task-b")
			require.NoError(t, err)
			require.Equal(t, before, after, "cross-tenant mutation changed task-b")
		})
	}
}

func seedProductImageTenantTasks(t *testing.T, newRepository func(*testing.T) productimage.TaskRepository) productimage.TaskRepository {
	t.Helper()
	repo := newRepository(t)
	for _, task := range []*productimage.Task{
		{
			ID: "task-a", Status: productimage.TaskStatusPending, TenantID: "tenant-b", UserID: "legacy-user-b",
			PersistedExecutionEnvelope: productImageExecutionEnvelope("tenant-a"),
		},
		{
			ID: "task-b", Status: productimage.TaskStatusPending, TenantID: "tenant-a", UserID: "legacy-user-a",
			PersistedExecutionEnvelope: productImageExecutionEnvelope("tenant-b"),
		},
	} {
		require.NoError(t, repo.CreateTask(context.Background(), task))
	}
	return repo
}

func seedProductImageSpacedTenantTask(t *testing.T, newRepository func(*testing.T) productimage.TaskRepository) productimage.TaskRepository {
	t.Helper()
	repo := newRepository(t)
	require.NoError(t, repo.CreateTask(context.Background(), &productimage.Task{
		ID:                         "task-spaced",
		Status:                     productimage.TaskStatusPending,
		TenantID:                   "legacy-tenant",
		PersistedExecutionEnvelope: productImageExecutionEnvelope(" tenant-a "),
	}))
	return repo
}

func seedProductImageNoncanonicalTenantTask(t *testing.T, newRepository func(*testing.T) productimage.TaskRepository, tenantID string) productimage.TaskRepository {
	t.Helper()
	repo := newRepository(t)
	require.NoError(t, repo.CreateTask(context.Background(), &productimage.Task{
		ID:                         "task-noncanonical",
		Status:                     productimage.TaskStatusPending,
		TenantID:                   "legacy-tenant",
		PersistedExecutionEnvelope: productImageExecutionEnvelope(tenantID),
	}))
	return repo
}

func productImageExecutionEnvelope(tenantID string) aiidentity.PersistedExecutionEnvelope {
	return aiidentity.PersistedExecutionEnvelope{
		ExecutionIdentityVersion: 1,
		ExecutionTenantID:        tenantID,
		ExecutionUserID:          "user-" + tenantID,
		ExecutionSourcePlatform:  "productimage",
		ExecutionSourceTaskType:  "image",
	}
}

func newProductImageGORMRepository(t *testing.T) productimage.TaskRepository {
	t.Helper()
	registerProductImageTenantScopeSQLite.Do(func() {
		sql.Register("productimage_tenant_scope_sqlite", &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("NOW", func() string { return "2026-08-22 00:00:00" }, true)
		}})
	})
	db, err := gorm.Open(sqlite.Dialector{DriverName: "productimage_tenant_scope_sqlite", DSN: ":memory:"}, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&productimage.Task{}))
	return NewTaskRepository(db)
}

var registerProductImageTenantScopeSQLite sync.Once
