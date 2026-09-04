package listingkit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit/core"
)

func TestPersistLayerFailureMarksTaskTerminal(t *testing.T) {
	repo := NewInMemoryRepositoryForTest()
	task := &Task{ID: "task-1", Status: core.TaskStatusProcessing}
	require.NoError(t, repo.CreateTask(context.Background(), task))
	svc := &service{repo: repo}

	require.NoError(t, svc.PersistLayerFailure(context.Background(), task.ID, "build amazon draft"))

	stored, err := repo.GetTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, core.TaskStatusFailed, stored.Status)
	require.Equal(t, "build amazon draft", stored.Error)
}

func TestPersistLayerFailureReturnsRepositoryError(t *testing.T) {
	want := errors.New("database unavailable")
	svc := &service{repo: layerFailureErrorRepository{Repository: NewInMemoryRepositoryForTest(), err: want}}

	err := svc.PersistLayerFailure(context.Background(), "task-1", "build amazon draft")

	require.ErrorIs(t, err, want)
}

func TestPersistLayerFailureDoesNotOverwriteCompletedTaskAfterLostResponse(t *testing.T) {
	repo := NewInMemoryRepositoryForTest()
	task := &Task{ID: "task-completed", Status: core.TaskStatusCompleted, Result: &ListingKitResult{Status: string(core.TaskStatusCompleted)}}
	require.NoError(t, repo.CreateTask(context.Background(), task))
	svc := &service{repo: repo}

	require.NoError(t, svc.PersistLayerFailure(context.Background(), task.ID, "activity response lost"))

	stored, err := repo.GetTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, core.TaskStatusCompleted, stored.Status)
	require.Empty(t, stored.Error)
}

type layerFailureErrorRepository struct {
	Repository
	err error
}

func (r layerFailureErrorRepository) MarkFailed(context.Context, string, string) error {
	return r.err
}

func (r layerFailureErrorRepository) MarkFailedIfProcessing(context.Context, string, string) (bool, error) {
	return false, r.err
}
