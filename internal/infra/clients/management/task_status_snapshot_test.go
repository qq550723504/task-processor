package management

import (
	"reflect"
	"testing"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/taskstatus"
)

func TestTaskStatusSnapshotFromDTOCopiesTaskStatusFields(t *testing.T) {
	status := &api.TaskStatusRespDTO{
		TaskID:           42,
		Status:           "processing",
		StatusKey:        "PROCESSING",
		StatusName:       "Processing",
		CanonicalStatus:  "running",
		Platform:         "SHEIN",
		Region:           "US",
		TaskType:         "listing",
		Priority:         7,
		RetryCount:       2,
		MaxRetries:       5,
		ProcessingTimeMs: 1234,
		QueueName:        "q-listing",
		ProcessingNode:   "node-a",
		ProgressPercent:  66,
		Result:           "partial",
		ErrorMessage:     "boom",
		ErrorStack:       "stack",
		ExecutionLogs:    []string{"a", "b"},
		TaskDetails:      "details",
	}

	got := taskStatusSnapshotFromDTO(status)
	want := &taskstatus.TaskStatusSnapshot{
		TaskID:           42,
		Status:           "processing",
		StatusKey:        "PROCESSING",
		StatusName:       "Processing",
		CanonicalStatus:  "running",
		Platform:         "SHEIN",
		Region:           "US",
		TaskType:         "listing",
		Priority:         7,
		RetryCount:       2,
		MaxRetries:       5,
		ProcessingTimeMs: 1234,
		QueueName:        "q-listing",
		ProcessingNode:   "node-a",
		ProgressPercent:  66,
		Result:           "partial",
		ErrorMessage:     "boom",
		ErrorStack:       "stack",
		ExecutionLogs:    []string{"a", "b"},
		TaskDetails:      "details",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("taskStatusSnapshotFromDTO() = %+v, want %+v", got, want)
	}
}

func TestTaskStatusSnapshotFromDTONil(t *testing.T) {
	if got := taskStatusSnapshotFromDTO(nil); got != nil {
		t.Fatalf("taskStatusSnapshotFromDTO(nil) = %+v, want nil", got)
	}
}
