package submission

import (
	"testing"
	"time"
)

func TestNewRemoteCompletionStateCarriesTaskPackageActionRequestAndResponse(t *testing.T) {
	t.Parallel()

	task := "task"
	pkg := "package"
	response := "response"
	startedAt := time.Date(2026, 6, 26, 11, 45, 0, 0, time.UTC)

	state := NewRemoteCompletionState(RemoteCompletionStateInput[string, string, string]{
		TaskID:    "task-1",
		Task:      task,
		Package:   pkg,
		Action:    "publish",
		RequestID: "request-1",
		StartedAt: startedAt,
		Response:  response,
	})

	if state.TaskID != "task-1" || state.Task != task || state.Package != pkg {
		t.Fatalf("identity fields = %+v", state)
	}
	if state.Action != "publish" || state.RequestID != "request-1" {
		t.Fatalf("request fields = %+v", state)
	}
	if !state.StartedAt.Equal(startedAt) || state.Response != response {
		t.Fatalf("completion fields = %+v", state)
	}
}
