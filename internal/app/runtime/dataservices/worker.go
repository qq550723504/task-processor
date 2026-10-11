package dataservicesruntime

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"task-processor/internal/product/dataacquisition"
	"time"
)

type Worker interface {
	Start() error
	Stop()
}

// Runtime owns start/stop, and installs the existing frozen workflow only.
func NewWorker(c client.Client, runner JobRunner) (Worker, error) {
	if c == nil || runner == nil {
		return nil, dataacquisition.ErrUnavailable
	}
	w := worker.New(c, DataTaskQueue, worker.Options{MaxConcurrentActivityExecutionSize: 2, MaxConcurrentWorkflowTaskExecutionSize: 2, WorkerStopTimeout: 10 * time.Second})
	if err := RegisterWorker(w, runner); err != nil {
		return nil, err
	}
	return w, nil
}
