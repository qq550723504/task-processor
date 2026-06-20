package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var errStudioBatchRunItemStillRunning = errors.New("studio batch run item still running")

type studioBatchRunItemStillRunningError struct {
	AsyncJobID string
}

func (e *studioBatchRunItemStillRunningError) Error() string {
	return errStudioBatchRunItemStillRunning.Error()
}

func (e *studioBatchRunItemStillRunningError) Unwrap() error {
	return errStudioBatchRunItemStillRunning
}

type taskStudioBatchRunExecutorConfig struct {
	repo               StudioBatchRunRepository
	executeGenerateOne func(ctx context.Context, batchID string) error
	executeCreateTasks func(ctx context.Context, batchID string) error
	now                func() time.Time
	waitStillRunning   func(context.Context) error
	completionRunner   *listingStudioBatchRunCompletionRunner
}

type taskStudioBatchRunExecutor struct {
	repo               StudioBatchRunRepository
	executeGenerateOne func(ctx context.Context, batchID string) error
	executeCreateTasks func(ctx context.Context, batchID string) error
	now                func() time.Time
	waitStillRunning   func(context.Context) error
	completionRunner   *listingStudioBatchRunCompletionRunner
}

func newTaskStudioBatchRunExecutor(config taskStudioBatchRunExecutorConfig) *taskStudioBatchRunExecutor {
	return &taskStudioBatchRunExecutor{
		repo:               config.repo,
		executeGenerateOne: config.executeGenerateOne,
		executeCreateTasks: config.executeCreateTasks,
		now:                config.now,
		waitStillRunning:   config.waitStillRunning,
		completionRunner:   config.completionRunner,
	}
}

func (e *taskStudioBatchRunExecutor) Run(ctx context.Context, runID string) error {
	if e == nil || e.repo == nil {
		return fmt.Errorf("studio batch run repository is not configured")
	}
	if e.executeGenerateOne == nil && e.executeCreateTasks == nil {
		return fmt.Errorf("studio batch run executor is not configured")
	}

	run, err := e.repo.GetStudioBatchRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	ctx = withStudioBatchRunIdentity(ctx, run)
	items, err := e.repo.ListStudioBatchRunItems(ctx, run.ID)
	if err != nil {
		return err
	}
	if run.CancelRequested {
		if err := e.cancelUnfinishedItems(ctx, items); err != nil {
			return err
		}
		return e.finalizeRun(ctx, run, items, StudioBatchRunStatusCancelled, "")
	}

	if run.StartedAt == nil {
		startedAt := e.currentTime()
		run.StartedAt = &startedAt
	}
	run.Status = StudioBatchRunStatusRunning
	e.refreshRunCounters(run, items)
	if err := e.repo.UpdateStudioBatchRun(ctx, run); err != nil {
		return err
	}

	for index := range items {
		item := &items[index]
		if item.Status == StudioBatchRunItemStatusSucceeded || item.Status == StudioBatchRunItemStatusFailed || item.Status == StudioBatchRunItemStatusCancelled {
			continue
		}

		run, err = e.repo.GetStudioBatchRun(ctx, run.ID)
		if err != nil {
			return err
		}
		if run.CancelRequested {
			if err := e.cancelUnfinishedItems(ctx, items); err != nil {
				return err
			}
			return e.finalizeRun(ctx, run, items, StudioBatchRunStatusCancelled, run.LastError)
		}

		run.Status = StudioBatchRunStatusRunning
		run.CurrentBatchID = item.BatchID
		run.CurrentIndex = item.Position
		if run.StartedAt == nil {
			startedAt := e.currentTime()
			run.StartedAt = &startedAt
		}

		if item.Status != StudioBatchRunItemStatusRunning {
			startedAt := e.currentTime()
			item.Status = StudioBatchRunItemStatusRunning
			item.StartedAt = &startedAt
			item.FinishedAt = nil
			item.ErrorMessage = ""
			item.UpdatedAt = startedAt
			if err := e.repo.UpdateStudioBatchRunItem(ctx, item); err != nil {
				return err
			}
		}
		run.UpdatedAt = e.currentTime()
		if err := e.repo.UpdateStudioBatchRun(ctx, run); err != nil {
			return err
		}

		for {
			execErr := e.executeOneForMode(ctx, run.Mode, item.BatchID)
			var stillRunningErr *studioBatchRunItemStillRunningError
			if errors.As(execErr, &stillRunningErr) {
				item.AsyncJobID = firstNonEmpty(strings.TrimSpace(stillRunningErr.AsyncJobID), item.AsyncJobID)
				item.FinishedAt = nil
				item.ErrorMessage = ""
				item.UpdatedAt = e.currentTime()
				if err := e.repo.UpdateStudioBatchRunItem(ctx, item); err != nil {
					return err
				}
				run.Status = StudioBatchRunStatusRunning
				run.LastError = ""
				e.refreshRunCounters(run, items)
				run.UpdatedAt = e.currentTime()
				if err := e.repo.UpdateStudioBatchRun(ctx, run); err != nil {
					return err
				}
				if err := e.waitForStillRunning(ctx); err != nil {
					return err
				}
				run, err = e.repo.GetStudioBatchRun(ctx, run.ID)
				if err != nil {
					return err
				}
				if run.CancelRequested {
					if err := e.cancelUnfinishedItems(ctx, items); err != nil {
						return err
					}
					return e.finalizeRun(ctx, run, items, StudioBatchRunStatusCancelled, run.LastError)
				}
				continue
			}
			finishedAt := e.currentTime()
			item.FinishedAt = &finishedAt
			item.UpdatedAt = finishedAt
			if execErr != nil {
				item.Status = StudioBatchRunItemStatusFailed
				item.ErrorMessage = execErr.Error()
			} else {
				item.Status = StudioBatchRunItemStatusSucceeded
				item.ErrorMessage = ""
			}
			if err := e.repo.UpdateStudioBatchRunItem(ctx, item); err != nil {
				return err
			}

			if item.ErrorMessage != "" {
				run.LastError = item.ErrorMessage
			}
			e.refreshRunCounters(run, items)
			run.CurrentBatchID = ""
			run.CurrentIndex = 0
			run.UpdatedAt = finishedAt

			if execErr != nil && run.FailurePolicy == StudioBatchRunFailurePolicyStopOnError {
				if err := e.finalizeRun(ctx, run, items, StudioBatchRunStatusFailed, execErr.Error()); err != nil {
					return err
				}
				return execErr
			}
			if err := e.repo.UpdateStudioBatchRun(ctx, run); err != nil {
				return err
			}
			break
		}
	}

	finalStatus := e.resolveFinalRunStatus(run, items)
	return e.finalizeRun(ctx, run, items, finalStatus, run.LastError)
}

func withStudioBatchRunIdentity(ctx context.Context, run *StudioBatchRunRecord) context.Context {
	if run == nil {
		return ctx
	}
	identity := RequestIdentityFromContext(ctx)
	if strings.TrimSpace(identity.TenantID) == "" {
		identity.TenantID = strings.TrimSpace(run.TenantID)
	}
	if strings.TrimSpace(identity.UserID) == "" {
		identity.UserID = strings.TrimSpace(run.UserID)
	}
	if strings.TrimSpace(identity.TenantID) != "" {
		ctx = WithTenantID(ctx, identity.TenantID)
	}
	return WithRequestIdentity(ctx, identity)
}

func (e *taskStudioBatchRunExecutor) executeOneForMode(ctx context.Context, mode StudioBatchRunMode, batchID string) error {
	switch mode {
	case "", StudioBatchRunModeGenerate:
		if e.executeGenerateOne == nil {
			return fmt.Errorf("studio batch generation executor is not configured")
		}
		return e.executeGenerateOne(ctx, batchID)
	case StudioBatchRunModeCreateTasks:
		if e.executeCreateTasks == nil {
			return fmt.Errorf("studio batch task executor is not configured")
		}
		return e.executeCreateTasks(ctx, batchID)
	default:
		return fmt.Errorf("studio batch run mode %s is not supported", mode)
	}
}

func (e *taskStudioBatchRunExecutor) cancelUnfinishedItems(ctx context.Context, items []StudioBatchRunItemRecord) error {
	e.ensureCompletionRunner()
	if e.completionRunner == nil {
		return fmt.Errorf("studio batch run completion service is not configured")
	}
	return e.completionRunner.CancelUnfinishedItems(ctx, items)
}

func (e *taskStudioBatchRunExecutor) resolveFinalRunStatus(run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) StudioBatchRunStatus {
	e.ensureCompletionRunner()
	if e.completionRunner == nil {
		return StudioBatchRunStatusSucceeded
	}
	return e.completionRunner.ResolveFinalStatus(run != nil && run.CancelRequested, items)
}

func (e *taskStudioBatchRunExecutor) finalizeRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord, status StudioBatchRunStatus, lastError string) error {
	if run == nil {
		return nil
	}
	finishedAt := e.currentTime()
	run.Status = status
	run.CurrentBatchID = ""
	run.CurrentIndex = 0
	run.LastError = strings.TrimSpace(lastError)
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt
	e.refreshRunCounters(run, items)
	return e.repo.UpdateStudioBatchRun(ctx, run)
}

func (e *taskStudioBatchRunExecutor) refreshRunCounters(run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) {
	if run == nil {
		return
	}

	e.ensureCompletionRunner()
	if e.completionRunner == nil {
		run.TotalBatches = len(items)
		return
	}
	counters := e.completionRunner.CountItems(items)
	run.TotalBatches = counters.Total
	run.CompletedBatches = counters.Completed
	run.SucceededBatches = counters.Succeeded
	run.FailedBatches = counters.Failed
}

func (e *taskStudioBatchRunExecutor) currentTime() time.Time {
	if e != nil && e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func (e *taskStudioBatchRunExecutor) waitForStillRunning(ctx context.Context) error {
	if e != nil && e.waitStillRunning != nil {
		return e.waitStillRunning(ctx)
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *taskStudioBatchRunExecutor) ensureCompletionRunner() {
	if e == nil || e.completionRunner != nil || e.repo == nil {
		return
	}
	e.completionRunner = newListingStudioBatchRunCompletionService(e.repo, e.now)
}

func (s *service) executeStudioBatchRunItem(ctx context.Context, batchID string) error {
	if s == nil {
		return fmt.Errorf("listingkit service is not configured")
	}
	detail, err := s.taskStudioBatchOrDefault().ResumeStudioBatchGeneration(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return ErrStudioSessionNotFound
	}
	if detail.Batch.Status == StudioBatchStatusReviewReady {
		return nil
	}
	if isStudioBatchRunDetailStillRunning(detail) {
		return &studioBatchRunItemStillRunningError{AsyncJobID: resolveStudioBatchRunAsyncJobID(detail)}
	}
	return studioBatchRunDetailError(detail)
}

func (s *service) executeStudioBatchRunTaskCreation(ctx context.Context, batchID string) error {
	if s == nil {
		return fmt.Errorf("listingkit service is not configured")
	}
	detail, err := s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return ErrStudioSessionNotFound
	}
	designIDs := collectApprovedStudioBatchDesignIDs(detail)
	if len(designIDs) == 0 {
		return NewStudioBatchActionValidationError("approved design_ids is required")
	}
	result, err := s.taskStudioBatchOrDefault().CreateStudioBatchTasks(ctx, batchID, &CreateStudioBatchTasksRequest{
		DesignIDs: designIDs,
	})
	if err != nil {
		return err
	}
	if result == nil || result.Batch == nil {
		return ErrStudioSessionNotFound
	}
	if len(result.FailedTasks) > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(result.FailedTasks[0].Message))
	}
	if result.Batch.Status == StudioBatchStatusTasksCreated {
		return nil
	}
	return fmt.Errorf("studio batch %s task creation finished with status %s", result.Batch.ID, result.Batch.Status)
}

func collectApprovedStudioBatchDesignIDs(detail *StudioBatchDetail) []string {
	if detail == nil {
		return nil
	}
	designIDs := make([]string, 0)
	for _, item := range detail.Items {
		for _, design := range item.Designs {
			if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
				continue
			}
			designID := strings.TrimSpace(design.ID)
			if designID == "" {
				continue
			}
			designIDs = append(designIDs, designID)
		}
	}
	return normalizeStudioBatchDesignIDs(designIDs)
}

func isStudioBatchRunDetailStillRunning(detail *StudioBatchDetail) bool {
	if detail == nil || detail.Batch == nil {
		return false
	}
	switch detail.Batch.Status {
	case StudioBatchStatusGenerating, StudioBatchStatusPartiallyMaterialized:
		for _, item := range detail.Items {
			switch item.Item.Status {
			case StudioBatchItemStatusPending, StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization:
				return true
			}
		}
	}
	return false
}

func resolveStudioBatchRunAsyncJobID(detail *StudioBatchDetail) string {
	if detail == nil {
		return ""
	}
	for _, item := range detail.Items {
		switch item.Item.Status {
		case StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization:
		default:
			continue
		}
		for index := len(item.Attempts) - 1; index >= 0; index-- {
			attempt := item.Attempts[index]
			switch attempt.Status {
			case StudioGenerationAttemptStatusSubmitted, StudioGenerationAttemptStatusPolling, StudioGenerationAttemptStatusSucceeded, StudioGenerationAttemptStatusMaterialized:
				if jobID := strings.TrimSpace(attempt.UpstreamJobID); jobID != "" {
					return jobID
				}
			}
		}
	}
	return ""
}

func studioBatchRunDetailError(detail *StudioBatchDetail) error {
	if detail == nil || detail.Batch == nil {
		return ErrStudioSessionNotFound
	}
	for _, item := range detail.Items {
		if item.Item.LastError != "" {
			return fmt.Errorf("%s", item.Item.LastError)
		}
	}
	return fmt.Errorf("studio batch %s generation finished with status %s", detail.Batch.ID, detail.Batch.Status)
}

func buildStudioBatchRunDesignRequest(session *SheinStudioSession) *StudioDesignRequest {
	if session == nil {
		return nil
	}
	return &StudioDesignRequest{
		Prompt:                    session.Prompt,
		Count:                     parseStudioBatchRunStyleCount(session.StyleCount),
		VariationIntensity:        session.VariationIntensity,
		PrintableWidth:            session.PrintableWidth,
		PrintableHeight:           session.PrintableHeight,
		ProductReferenceImageURLs: studioBatchRunReferenceImageURLs(session),
		ImageModel:                session.ArtworkModel,
		TransparentBackground:     session.TransparentBackground,
	}
}

func buildStudioBatchRunDesigns(execution *StudioBatchGenerateExecutionOutput) []SheinStudioDesign {
	if execution == nil || execution.Response == nil {
		return nil
	}
	designs := make([]SheinStudioDesign, 0, len(execution.Response.Images))
	for index, image := range execution.Response.Images {
		designs = append(designs, SheinStudioDesign{
			ID:                    strings.TrimSpace(image.ID),
			SessionID:             strings.TrimSpace(execution.SessionID),
			ImageURL:              strings.TrimSpace(image.ImageURL),
			Prompt:                strings.TrimSpace(image.Prompt),
			RevisedPrompt:         strings.TrimSpace(image.RevisedPrompt),
			ImageModel:            strings.TrimSpace(image.ImageModel),
			TransparentBackground: image.TransparentBackground,
			VariationIntensity:    strings.TrimSpace(image.VariationIntensity),
			Role:                  strings.TrimSpace(image.Role),
			RoleLabel:             strings.TrimSpace(image.RoleLabel),
			SortOrder:             index,
		})
	}
	return designs
}

func studioBatchRunReferenceImageURLs(session *SheinStudioSession) []string {
	if session == nil {
		return nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, len(session.Selection.MockupImageURLs)+len(session.Selection.SizeReferenceImageURLs)+len(session.SelectedSDSImages)+2)
	appendURL := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	appendURL(session.Selection.MockupImageURL)
	for _, value := range session.Selection.MockupImageURLs {
		appendURL(value)
	}
	for _, value := range session.Selection.SizeReferenceImageURLs {
		appendURL(value)
	}
	for _, image := range session.SelectedSDSImages {
		appendURL(image.ImageURL)
	}
	return result
}

func parseStudioBatchRunStyleCount(value string) int {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || count <= 0 {
		return 1
	}
	return count
}
