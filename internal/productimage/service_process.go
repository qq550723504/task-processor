package productimage

import (
	"context"
	"fmt"
	"strings"
	"time"

	corelogger "task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

func (s *service) ProcessImages(ctx context.Context, task *Task) (*ImageProcessResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	log := loggerForImageProcess(task.ID)
	startedAt := time.Now()
	log.WithFields(logrus.Fields{
		"marketplace": task.Request.Marketplace,
		"retry_count": task.RetryCount,
		"status":      task.Status,
	}).Info("starting productimage processing")

	if err := s.taskRepo.MarkProcessing(ctx, task.ID); err != nil {
		log.WithError(err).Error("failed to mark image task as processing")
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	state := &PipelineState{Task: task, Result: task.Result}
	if err := s.runPipeline(ctx, state); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"outcome":     "failed",
		}).Error("productimage pipeline failed")
		logPipelineState(log, state)
		if dbErr := s.taskRepo.MarkFailed(ctx, task.ID, err.Error()); dbErr != nil {
			return nil, fmt.Errorf("pipeline failed: %v; additionally failed to persist error: %w", err, dbErr)
		}
		if s.cleanupTemporaryFiles {
			cleanupTemporaryAssets(state.Result)
		}
		return nil, err
	}

	if state.Result != nil && state.Result.Review != nil && state.Result.Review.NeedsReview {
		if s.cleanupTemporaryFiles {
			cleanupTemporaryAssets(state.Result)
		}
		reason := strings.Join(state.Result.Review.Reasons, "; ")
		if err := s.taskRepo.MarkNeedsReview(ctx, task.ID, state.Result, reason); err != nil {
			log.WithError(err).Error("failed to save needs_review image task result")
			_ = s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to save review result: %v", err))
			return nil, fmt.Errorf("failed to save needs_review task result: %w", err)
		}
		logPipelineState(log, state)
		log.WithFields(logrus.Fields{
			"duration_ms":    time.Since(startedAt).Milliseconds(),
			"outcome":        "needs_review",
			"review_reasons": state.Result.Review.Reasons,
		}).Warn("productimage task requires manual review")
		return state.Result, nil
	}

	if s.cleanupTemporaryFiles {
		cleanupTemporaryAssets(state.Result)
	}
	if err := s.taskRepo.MarkCompleted(ctx, task.ID, state.Result); err != nil {
		log.WithError(err).Error("failed to save image task result")
		_ = s.taskRepo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to save result: %v", err))
		return nil, fmt.Errorf("failed to save task result: %w", err)
	}

	logPipelineState(log, state)
	log.WithFields(logrus.Fields{
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"outcome":     "success",
	}).Info("productimage task completed successfully")
	return state.Result, nil
}

func loggerForImageProcess(taskID string) *logrus.Entry {
	return corelogger.GetGlobalLogger("productimage/service_process.go").WithField("task_id", taskID)
}

func logPipelineState(log *logrus.Entry, state *PipelineState) {
	if log == nil || state == nil || state.Result == nil {
		return
	}
	for _, summary := range state.Result.StageSummaries {
		log.WithFields(logrus.Fields{
			"stage":       summary.Stage,
			"outcome":     summary.Outcome,
			"duration_ms": summary.DurationMS,
			"message":     summary.Message,
		}).Info("productimage stage summary")
	}
	for _, trace := range state.Result.ImageTraces {
		log.WithFields(logrus.Fields{
			"stage":       trace.Stage,
			"image_url":   trace.ImageURL,
			"asset_type":  trace.AssetType,
			"outcome":     trace.Outcome,
			"duration_ms": trace.DurationMS,
			"message":     trace.Message,
		}).Info("productimage image trace")
	}
}
