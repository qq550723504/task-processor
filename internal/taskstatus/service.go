package taskstatus

import (
	"fmt"

	apptaskstatus "task-processor/internal/app/taskstatus"
	"task-processor/internal/listingruntime"
)

type ImportTaskStatusClient = apptaskstatus.ImportTaskStatusClient
type UpdateInput = apptaskstatus.UpdateInput
type Service = apptaskstatus.Service

type TaskStatusSnapshot struct {
	TaskID           int64
	Status           string
	StatusKey        string
	StatusName       string
	CanonicalStatus  string
	Platform         string
	Region           string
	TaskType         string
	Priority         int
	RetryCount       int
	MaxRetries       int
	ProcessingTimeMs int64
	QueueName        string
	ProcessingNode   string
	ProgressPercent  int
	Result           string
	ErrorMessage     string
	ErrorStack       string
	ExecutionLogs    []string
	TaskDetails      string
}

type RuntimeTaskStatusUpdater interface {
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
}

type RuntimeWithTaskRPC interface {
	RuntimeTaskStatusUpdater
	GetTaskStatus(taskID int64) (*TaskStatusSnapshot, error)
}

func NewService(component string, clientProvider func() ImportTaskStatusClient) *Service {
	return apptaskstatus.NewService(component, clientProvider)
}

type runtimeTaskStatusAdapter struct {
	client RuntimeTaskStatusUpdater
}

func NewRuntimeTaskStatusAdapter(client RuntimeTaskStatusUpdater) ImportTaskStatusClient {
	if client == nil {
		return nil
	}
	return runtimeTaskStatusAdapter{client: client}
}

func (a runtimeTaskStatusAdapter) UpdateTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if a.client == nil {
		return fmt.Errorf("task status runtime is not initialized")
	}
	return a.client.UpdateRuntimeTaskStatus(req)
}
