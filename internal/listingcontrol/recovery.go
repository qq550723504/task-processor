package listingcontrol

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/listingadmin"
)

const (
	defaultProcessingTimeoutMinutes  = 30
	defaultProcessingRecoveryLimit   = 100
	defaultStaleQueuedTimeoutMinutes = 120
	defaultStaleQueuedRecoveryLimit  = 500
)

type RecoveryRepository interface {
	CountTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time) (int64, error)
	ListTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]listingadmin.ImportTask, error)
	RecoverTimedOutProcessingTasks(ctx context.Context, ids []int64, recovery listingadmin.ProcessingTimeoutRecovery) (int, error)
	CountStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time) (int64, error)
	ListStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]listingadmin.ImportTask, error)
	RecoverStaleQueuedTasks(ctx context.Context, ids []int64, recovery listingadmin.StaleQueuedRecovery) (int, error)
}

type RecoveryConfig struct {
	Enabled                   bool
	ProcessingTimeoutEnabled  bool
	ProcessingTimeoutMinutes  int
	ProcessingRecoveryLimit   int
	StaleQueuedEnabled        bool
	StaleQueuedTimeoutMinutes int
	StaleQueuedRecoveryLimit  int
	Repository                RecoveryRepository
	Now                       func() time.Time
}

type RecoverySummary struct {
	ProcessingCandidates  int     `json:"processingCandidates"`
	ProcessingRecovered   int     `json:"processingRecovered"`
	ProcessingTaskIDs     []int64 `json:"processingTaskIds,omitempty"`
	StaleQueuedCandidates int     `json:"staleQueuedCandidates"`
	StaleQueuedRecovered  int     `json:"staleQueuedRecovered"`
	StaleQueuedTaskIDs    []int64 `json:"staleQueuedTaskIds,omitempty"`
}

type RecoveryCoordinator struct {
	config RecoveryConfig
}

func NewRecoveryCoordinator(cfg RecoveryConfig) *RecoveryCoordinator {
	return &RecoveryCoordinator{config: normalizeRecoveryConfig(cfg)}
}

func (c *RecoveryCoordinator) RunOnce(ctx context.Context) (RecoverySummary, error) {
	var summary RecoverySummary
	if c == nil {
		return summary, fmt.Errorf("recovery coordinator is nil")
	}
	cfg := c.config
	if !cfg.Enabled {
		return summary, nil
	}
	if cfg.Repository == nil {
		return summary, fmt.Errorf("recovery repository is nil")
	}

	now := cfg.Now()
	if cfg.ProcessingTimeoutEnabled {
		processingSummary, err := c.recoverProcessingTimeout(ctx, now)
		summary.ProcessingCandidates = processingSummary.ProcessingCandidates
		summary.ProcessingRecovered = processingSummary.ProcessingRecovered
		summary.ProcessingTaskIDs = processingSummary.ProcessingTaskIDs
		if err != nil {
			return summary, err
		}
	}
	if cfg.StaleQueuedEnabled {
		staleSummary, err := c.recoverStaleQueued(ctx, now)
		summary.StaleQueuedCandidates = staleSummary.StaleQueuedCandidates
		summary.StaleQueuedRecovered = staleSummary.StaleQueuedRecovered
		summary.StaleQueuedTaskIDs = staleSummary.StaleQueuedTaskIDs
		if err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (c *RecoveryCoordinator) recoverProcessingTimeout(ctx context.Context, now time.Time) (RecoverySummary, error) {
	var summary RecoverySummary
	cfg := c.config
	timeoutBefore := now.Add(-time.Duration(cfg.ProcessingTimeoutMinutes) * time.Minute)
	count, err := cfg.Repository.CountTimedOutProcessingTasks(ctx, timeoutBefore)
	if err != nil {
		return summary, fmt.Errorf("count timed out processing tasks: %w", err)
	}
	summary.ProcessingCandidates = int(count)

	tasks, err := cfg.Repository.ListTimedOutProcessingTasks(ctx, timeoutBefore, cfg.ProcessingRecoveryLimit)
	if err != nil {
		return summary, fmt.Errorf("list timed out processing tasks: %w", err)
	}
	summary.ProcessingTaskIDs = taskIDs(tasks)
	if len(summary.ProcessingTaskIDs) == 0 {
		return summary, nil
	}

	recovered, err := cfg.Repository.RecoverTimedOutProcessingTasks(ctx, summary.ProcessingTaskIDs, listingadmin.ProcessingTimeoutRecovery{
		TimeoutMinutes: cfg.ProcessingTimeoutMinutes,
		TimeoutBefore:  timeoutBefore,
		ErrorMessage:   "Task processing lease expired, recovered by listing control plane",
		ReasonCode:     "PROCESSING_TIMEOUT",
		Stage:          "processing_timeout_recovery",
		Remark:         fmt.Sprintf("Recovered after processing timeout by listing control plane (%d minutes)", cfg.ProcessingTimeoutMinutes),
	})
	if err != nil {
		return summary, fmt.Errorf("recover timed out processing tasks: %w", err)
	}
	summary.ProcessingRecovered = recovered
	return summary, nil
}

func (c *RecoveryCoordinator) recoverStaleQueued(ctx context.Context, now time.Time) (RecoverySummary, error) {
	var summary RecoverySummary
	cfg := c.config
	timeoutBefore := now.Add(-time.Duration(cfg.StaleQueuedTimeoutMinutes) * time.Minute)
	count, err := cfg.Repository.CountStaleQueuedTasks(ctx, timeoutBefore)
	if err != nil {
		return summary, fmt.Errorf("count stale queued tasks: %w", err)
	}
	summary.StaleQueuedCandidates = int(count)

	tasks, err := cfg.Repository.ListStaleQueuedTasks(ctx, timeoutBefore, cfg.StaleQueuedRecoveryLimit)
	if err != nil {
		return summary, fmt.Errorf("list stale queued tasks: %w", err)
	}
	summary.StaleQueuedTaskIDs = taskIDs(tasks)
	if len(summary.StaleQueuedTaskIDs) == 0 {
		return summary, nil
	}

	recovered, err := cfg.Repository.RecoverStaleQueuedTasks(ctx, summary.StaleQueuedTaskIDs, listingadmin.StaleQueuedRecovery{
		TimeoutMinutes: cfg.StaleQueuedTimeoutMinutes,
		TimeoutBefore:  timeoutBefore,
		ErrorMessage:   "Task stayed queued too long, recovered by listing control plane",
		ReasonCode:     "STALE_QUEUED",
		Stage:          "queued_timeout_recovery",
		Remark:         fmt.Sprintf("Recovered from stale queued state by listing control plane (%d minutes)", cfg.StaleQueuedTimeoutMinutes),
	})
	if err != nil {
		return summary, fmt.Errorf("recover stale queued tasks: %w", err)
	}
	summary.StaleQueuedRecovered = recovered
	return summary, nil
}

func normalizeRecoveryConfig(cfg RecoveryConfig) RecoveryConfig {
	if cfg.ProcessingTimeoutMinutes <= 0 {
		cfg.ProcessingTimeoutMinutes = defaultProcessingTimeoutMinutes
	}
	if cfg.ProcessingRecoveryLimit <= 0 {
		cfg.ProcessingRecoveryLimit = defaultProcessingRecoveryLimit
	}
	if cfg.StaleQueuedTimeoutMinutes <= 0 {
		cfg.StaleQueuedTimeoutMinutes = defaultStaleQueuedTimeoutMinutes
	}
	if cfg.StaleQueuedRecoveryLimit <= 0 {
		cfg.StaleQueuedRecoveryLimit = defaultStaleQueuedRecoveryLimit
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

func taskIDs(tasks []listingadmin.ImportTask) []int64 {
	ids := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}
