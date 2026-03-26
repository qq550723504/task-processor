package productenrich

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"task-processor/internal/core/logger"
)

func (s *productService) ProcessProduct(ctx context.Context, task *Task) (*ProductJSON, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	log := loggerForServiceProcess(task.ID)
	startedAt := time.Now()
	log.WithFields(logrus.Fields{
		"capability_mode": s.capabilities.Mode,
		"retry_count":     task.RetryCount,
		"status":          task.Status,
	}).Info("starting product processing")

	if err := s.taskRepo.MarkProcessing(ctx, task.ID); err != nil {
		log.WithError(err).Error("failed to mark task as processing")
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	state := &PipelineState{Task: task}
	if err := s.runPipeline(ctx, state); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"duration_ms":         time.Since(startedAt).Milliseconds(),
			"failure_disposition": ClassifyProcessFailure(err),
			"outcome":             "failed",
		}).Error("product processing failed")
		return nil, err
	}

	log.Info("saving task result")
	if err := s.taskRepo.MarkCompleted(ctx, task.ID, state.ProductJSON); err != nil {
		log.WithError(err).Error("failed to save task result")
		if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to save result: %v", err)); dbErr != nil {
			log.WithError(dbErr).Error("failed to persist task error")
		}
		return nil, fmt.Errorf("failed to save task result: %w", err)
	}

	log.WithFields(logrus.Fields{
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"outcome":     "success",
	}).Info("task completed successfully")
	return state.ProductJSON, nil
}

func loggerForServiceProcess(taskID string) *logrus.Entry {
	return logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", taskID)
}
