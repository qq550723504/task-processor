package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type studioBatchRunCoordinatorConfig struct {
	repo       StudioBatchRunRepository
	recoverRun func(ctx context.Context, runID string) error
	executor   *taskStudioBatchRunExecutor
}

type studioBatchRunCoordinator struct {
	repo       StudioBatchRunRepository
	recoverRun func(ctx context.Context, runID string) error
}

func newStudioBatchRunCoordinator(config studioBatchRunCoordinatorConfig) *studioBatchRunCoordinator {
	recoverRun := config.recoverRun
	if recoverRun == nil && config.executor != nil {
		recoverRun = config.executor.Run
	}
	return &studioBatchRunCoordinator{
		repo:       config.repo,
		recoverRun: recoverRun,
	}
}

func (c *studioBatchRunCoordinator) RecoverUnfinishedRuns(ctx context.Context) error {
	if c == nil || c.repo == nil {
		return fmt.Errorf("studio batch run repository is not configured")
	}
	if c.recoverRun == nil {
		return fmt.Errorf("studio batch run recovery callback is not configured")
	}

	runs, err := c.repo.ListUnfinishedStudioBatchRuns(ctx)
	if err != nil {
		return err
	}

	var recoveryErrors []error
	for _, run := range runs {
		recoveryCtx := withStudioBatchRunIdentity(ctx, &run)
		if err := c.recoverRun(recoveryCtx, run.ID); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("recover studio batch run %s: %w", run.ID, err))
		}
	}
	return errors.Join(recoveryErrors...)
}

func (c *studioBatchRunCoordinator) StartRun(ctx context.Context, runID string) error {
	if c == nil {
		return fmt.Errorf("studio batch run coordinator is not configured")
	}
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return fmt.Errorf("studio batch run id is required")
	}
	if c.recoverRun == nil {
		return fmt.Errorf("studio batch run recovery callback is not configured")
	}

	detachedCtx := DetachedRequestContext(ctx)
	go func() {
		if err := c.recoverRun(detachedCtx, trimmedRunID); err != nil {
			logrus.WithFields(studioBatchRunLogFields(detachedCtx, logrus.Fields{
				"run_id": trimmedRunID,
			})).WithError(err).Warn("listingkit studio batch run execution failed")
		}
	}()
	return nil
}

func (s *service) initializeStudioBatchRunRecovery() {
	coordinator := s.studioBatchRunCoordinatorOrDefault()
	if coordinator == nil {
		return
	}
	go func() {
		if err := coordinator.RecoverUnfinishedRuns(context.Background()); err != nil {
			logrus.WithError(err).Warn("listingkit studio batch run recovery failed")
		}
	}()
}

func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {
	coordinator := s.studioBatchRunCoordinatorOrDefault()
	if coordinator == nil {
		return fmt.Errorf("studio batch run executor is not configured")
	}
	return coordinator.StartRun(ctx, runID)
}

func (s *service) initializeStudioBatchRunSupportCollaborators() {
	if s == nil {
		return
	}
	s.resolveTaskStudioBatchRunCollaborators()
}

func (s *service) buildStudioBatchRunCoordinator() *studioBatchRunCoordinator {
	return s.resolveTaskStudioBatchRunCollaborators().runCoordinator
}

func (s *service) buildStudioBatchRunExecutor() *taskStudioBatchRunExecutor {
	return s.resolveTaskStudioBatchRunCollaborators().runExecutor
}

func studioBatchRunLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {
	if fields == nil {
		fields = logrus.Fields{}
	}
	for key, value := range RequestTraceFromContext(ctx).LogFields() {
		if value == "" || value == 0 {
			continue
		}
		fields[key] = value
	}
	return fields
}
