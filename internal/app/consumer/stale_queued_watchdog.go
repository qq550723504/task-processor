package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/listingadmin"

	"github.com/sirupsen/logrus"
)

const (
	staleQueuedReasonCode   = "STALE_QUEUED"
	staleQueuedStage        = "queued_timeout_recovery"
	staleQueuedErrorMessage = "Task stayed queued too long, recovered by scheduler watchdog"
)

type StaleQueuedRepository interface {
	CountStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time) (int64, error)
	ListStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]listingadmin.ImportTask, error)
	RecoverStaleQueuedTasks(ctx context.Context, ids []int64, recovery listingadmin.StaleQueuedRecovery) (int, error)
}

type StaleQueuedWatchdogConfig struct {
	Enabled        bool
	Interval       time.Duration
	TimeoutMinutes int
	RecoveryLimit  int
	Repository     StaleQueuedRepository
	Logger         *logrus.Logger
}

type StaleQueuedSummary struct {
	Candidates int
	Recovered  int
	TaskIDs    []int64
}

type StaleQueuedWatchdog struct {
	cfg    StaleQueuedWatchdogConfig
	logger *logrus.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu          sync.RWMutex
	started     bool
	lastRunAt   time.Time
	lastError   string
	lastSummary StaleQueuedSummary
}

func NewStaleQueuedWatchdog(cfg StaleQueuedWatchdogConfig) *StaleQueuedWatchdog {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.TimeoutMinutes <= 0 {
		cfg.TimeoutMinutes = 120
	}
	if cfg.RecoveryLimit <= 0 {
		cfg.RecoveryLimit = 500
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	return &StaleQueuedWatchdog{
		cfg:    cfg,
		logger: cfg.Logger,
	}
}

func (w *StaleQueuedWatchdog) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return nil
	}
	if w.cfg.Enabled && w.cfg.Repository == nil {
		return fmt.Errorf("stale queued watchdog repository is nil")
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.started = true
	w.wg.Add(1)
	go w.loop()
	return nil
}

func (w *StaleQueuedWatchdog) Stop(ctx context.Context) error {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	cancel := w.cancel
	w.started = false
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *StaleQueuedWatchdog) GetStatus() map[string]any {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return map[string]any{
		"started":         w.started,
		"enabled":         w.cfg.Enabled,
		"interval":        w.cfg.Interval.String(),
		"timeout_minutes": w.cfg.TimeoutMinutes,
		"recovery_limit":  w.cfg.RecoveryLimit,
		"last_run_at":     w.lastRunAt,
		"last_error":      w.lastError,
		"last_candidates": w.lastSummary.Candidates,
		"last_recovered":  w.lastSummary.Recovered,
		"last_task_ids":   append([]int64(nil), w.lastSummary.TaskIDs...),
	}
}

func (w *StaleQueuedWatchdog) RunOnce(ctx context.Context) (StaleQueuedSummary, error) {
	summary, err := w.runOnce(ctx)
	w.record(time.Now(), summary, err)
	return summary, err
}

func (w *StaleQueuedWatchdog) loop() {
	defer w.wg.Done()

	_, _ = w.RunOnce(w.ctx)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.RunOnce(w.ctx)
		}
	}
}

func (w *StaleQueuedWatchdog) runOnce(ctx context.Context) (StaleQueuedSummary, error) {
	if !w.cfg.Enabled {
		return StaleQueuedSummary{}, nil
	}
	if w.cfg.Repository == nil {
		return StaleQueuedSummary{}, fmt.Errorf("stale queued watchdog repository is nil")
	}
	timeoutBefore := time.Now().Add(-time.Duration(w.cfg.TimeoutMinutes) * time.Minute)
	count, err := w.cfg.Repository.CountStaleQueuedTasks(ctx, timeoutBefore)
	if err != nil {
		return StaleQueuedSummary{}, err
	}
	tasks, err := w.cfg.Repository.ListStaleQueuedTasks(ctx, timeoutBefore, w.cfg.RecoveryLimit)
	if err != nil {
		return StaleQueuedSummary{}, err
	}
	ids := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		if task.ID > 0 {
			ids = append(ids, task.ID)
		}
	}
	summary := StaleQueuedSummary{
		Candidates: int(count),
		TaskIDs:    append([]int64(nil), ids...),
	}
	if len(ids) == 0 {
		w.logger.WithFields(logrus.Fields{
			"timeout_minutes": w.cfg.TimeoutMinutes,
			"candidates":      count,
		}).Debug("stale queued watchdog found no recoverable tasks")
		return summary, nil
	}

	recovered, err := w.cfg.Repository.RecoverStaleQueuedTasks(ctx, ids, listingadmin.StaleQueuedRecovery{
		TimeoutMinutes: w.cfg.TimeoutMinutes,
		ErrorMessage:   staleQueuedErrorMessage,
		ReasonCode:     staleQueuedReasonCode,
		Stage:          staleQueuedStage,
		Remark:         fmt.Sprintf("Recovered from stale queued state by scheduler watchdog (%d minutes)", w.cfg.TimeoutMinutes),
	})
	if err != nil {
		return summary, err
	}
	summary.Recovered = recovered
	w.logger.WithFields(logrus.Fields{
		"timeout_minutes": w.cfg.TimeoutMinutes,
		"candidates":      count,
		"selected":        len(ids),
		"recovered":       recovered,
		"task_ids":        ids,
	}).Info("stale queued watchdog recovered tasks")
	return summary, nil
}

func (w *StaleQueuedWatchdog) record(runAt time.Time, summary StaleQueuedSummary, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastRunAt = runAt
	w.lastSummary = summary
	if err != nil {
		w.lastError = err.Error()
		return
	}
	w.lastError = ""
}
