package preview

import (
	"slices"
	"testing"
	"time"
)

func TestBuildShell(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	completedAt := createdAt.Add(2 * time.Minute)
	shell := BuildShell(ShellInput{
		TaskID:           "task-1",
		Status:           "completed",
		SelectedPlatform: "shein",
		Platforms:        []string{"shein", "amazon"},
		CreatedAt:        createdAt,
		CompletedAt:      &completedAt,
	})
	if shell == nil {
		t.Fatal("shell = nil")
	}
	if shell.TaskID != "task-1" || shell.Status != "completed" {
		t.Fatalf("shell = %+v", shell)
	}
	if !slices.Equal(shell.Platforms, []string{"shein", "amazon"}) {
		t.Fatalf("platforms = %#v", shell.Platforms)
	}
	if shell.CompletedAt == nil || !shell.CompletedAt.Equal(completedAt) {
		t.Fatalf("completedAt = %+v", shell.CompletedAt)
	}
}

func TestBuildTaskShell(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	updatedAt := createdAt.Add(3 * time.Minute)
	shell := BuildTaskShell(TaskShellInput{
		TaskID:           "task-2",
		Status:           "completed",
		SelectedPlatform: "shein",
		ResultPlatforms:  []string{"shein", "temu"},
		RequestPlatforms: []string{"amazon"},
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	})
	if shell == nil {
		t.Fatal("shell = nil")
	}
	if shell.TaskID != "task-2" || shell.Status != "completed" {
		t.Fatalf("shell = %+v", shell)
	}
	if !slices.Equal(shell.Platforms, []string{"shein", "temu"}) {
		t.Fatalf("platforms = %#v", shell.Platforms)
	}
	if shell.CompletedAt == nil || !shell.CompletedAt.Equal(updatedAt) {
		t.Fatalf("completedAt = %+v", shell.CompletedAt)
	}
}

func TestBuildTaskShellLeavesCompletedAtEmptyForPendingStatus(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	shell := BuildTaskShell(TaskShellInput{
		TaskID:           "task-3",
		Status:           "processing",
		SelectedPlatform: "amazon",
		RequestPlatforms: []string{"amazon"},
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt.Add(time.Minute),
	})
	if shell == nil {
		t.Fatal("shell = nil")
	}
	if shell.CompletedAt != nil {
		t.Fatalf("completedAt = %+v, want nil", shell.CompletedAt)
	}
}
