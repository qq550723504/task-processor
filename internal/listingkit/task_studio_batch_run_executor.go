package listingkit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type taskStudioBatchRunExecutorConfig struct {
	repo       StudioBatchRunRepository
	executeOne func(ctx context.Context, batchID string) error
	now        func() time.Time
}

type taskStudioBatchRunExecutor struct {
	repo       StudioBatchRunRepository
	executeOne func(ctx context.Context, batchID string) error
	now        func() time.Time
}

func newTaskStudioBatchRunExecutor(config taskStudioBatchRunExecutorConfig) *taskStudioBatchRunExecutor {
	return &taskStudioBatchRunExecutor{
		repo:       config.repo,
		executeOne: config.executeOne,
		now:        config.now,
	}
}

func (e *taskStudioBatchRunExecutor) Run(ctx context.Context, runID string) error {
	if e == nil || e.repo == nil {
		return fmt.Errorf("studio batch run repository is not configured")
	}
	if e.executeOne == nil {
		return fmt.Errorf("studio batch run executor is not configured")
	}

	run, err := e.repo.GetStudioBatchRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
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

		execErr := e.executeOne(ctx, item.BatchID)
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
	}

	finalStatus := e.resolveFinalRunStatus(run, items)
	return e.finalizeRun(ctx, run, items, finalStatus, run.LastError)
}

func (e *taskStudioBatchRunExecutor) cancelUnfinishedItems(ctx context.Context, items []StudioBatchRunItemRecord) error {
	for index := range items {
		item := &items[index]
		switch item.Status {
		case StudioBatchRunItemStatusSucceeded, StudioBatchRunItemStatusFailed, StudioBatchRunItemStatusCancelled:
			continue
		}
		finishedAt := e.currentTime()
		item.Status = StudioBatchRunItemStatusCancelled
		item.FinishedAt = &finishedAt
		item.UpdatedAt = finishedAt
		if err := e.repo.UpdateStudioBatchRunItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (e *taskStudioBatchRunExecutor) resolveFinalRunStatus(run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) StudioBatchRunStatus {
	if run != nil && run.CancelRequested {
		return StudioBatchRunStatusCancelled
	}

	succeeded := 0
	failed := 0
	for _, item := range items {
		switch item.Status {
		case StudioBatchRunItemStatusSucceeded:
			succeeded++
		case StudioBatchRunItemStatusFailed:
			failed++
		}
	}

	switch {
	case failed > 0 && succeeded > 0:
		return StudioBatchRunStatusPartiallySucceeded
	case failed > 0:
		return StudioBatchRunStatusFailed
	default:
		return StudioBatchRunStatusSucceeded
	}
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

	run.TotalBatches = len(items)
	run.CompletedBatches = 0
	run.SucceededBatches = 0
	run.FailedBatches = 0
	for _, item := range items {
		switch item.Status {
		case StudioBatchRunItemStatusSucceeded:
			run.CompletedBatches++
			run.SucceededBatches++
		case StudioBatchRunItemStatusFailed, StudioBatchRunItemStatusCancelled:
			run.CompletedBatches++
			run.FailedBatches++
		}
	}
}

func (e *taskStudioBatchRunExecutor) currentTime() time.Time {
	if e != nil && e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
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
	return studioBatchRunDetailError(detail)
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
