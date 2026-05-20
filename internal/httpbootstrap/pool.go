package httpbootstrap

import (
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
)

type PoolSubmitter struct {
	Pool worker.WorkerPool
}

func (s *PoolSubmitter) Submit(taskID string) error {
	return s.Pool.Submit(worker.WorkerJob{TaskData: taskID})
}

func NewWorkerPool(processor worker.Processor, cfg *config.Config) worker.WorkerPool {
	return worker.NewPoolWithConfig(processor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
}
