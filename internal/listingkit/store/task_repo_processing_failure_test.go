package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/store"
)

type processingFailureRepository interface {
	MarkFailedIfProcessing(ctx context.Context, taskID string, errorMessage string) (bool, error)
}

func TestTaskRepositoriesFailOnlyProcessingTasks(t *testing.T) {
	factories := map[string]func(*testing.T) listingkit.Repository{
		"memory": func(*testing.T) listingkit.Repository { return store.NewMemTaskRepository() },
		"gorm": func(t *testing.T) listingkit.Repository {
			db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&listingkit.Task{}))
			return store.NewTaskRepository(db)
		},
	}
	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			repo := factory(t)
			conditional, ok := repo.(processingFailureRepository)
			require.True(t, ok, "repository must expose the processing-state CAS")
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")

			processing := &listingkit.Task{ID: "processing", TenantID: "tenant-a", Status: core.TaskStatusProcessing}
			completed := &listingkit.Task{ID: "completed", TenantID: "tenant-a", Status: core.TaskStatusCompleted}
			require.NoError(t, repo.CreateTask(ctx, processing))
			require.NoError(t, repo.CreateTask(ctx, completed))

			changed, err := conditional.MarkFailedIfProcessing(ctx, processing.ID, "build amazon draft")
			require.NoError(t, err)
			require.True(t, changed)
			stored, err := repo.GetTask(ctx, processing.ID)
			require.NoError(t, err)
			require.Equal(t, core.TaskStatusFailed, stored.Status)

			changed, err = conditional.MarkFailedIfProcessing(ctx, completed.ID, "activity response lost")
			require.NoError(t, err)
			require.False(t, changed)
			stored, err = repo.GetTask(ctx, completed.ID)
			require.NoError(t, err)
			require.Equal(t, core.TaskStatusCompleted, stored.Status)
			require.Empty(t, stored.Error)
		})
	}
}
