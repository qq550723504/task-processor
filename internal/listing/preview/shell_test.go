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
