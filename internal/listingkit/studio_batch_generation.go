package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	defaultStudioBatchTransientRetryLimit = 3
	defaultStudioBatchStaleRecoveryLimit  = defaultStudioBatchTransientRetryLimit + 1
	defaultStudioBatchAttemptStaleAfter   = 10 * time.Minute
)

type studioBatchGenerator interface {
	RunPendingStudioBatchItems(ctx context.Context, batchID string) error
	RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error
}

type studioBatchGenerateExecutor func(context.Context, StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error)
type studioBatchGenerateAsyncSubmitter func(context.Context, StudioBatchGenerateExecutionInput) (*AIImageAsyncSubmit, error)
type studioBatchGenerateAsyncQuerier func(context.Context, StudioBatchGenerateExecutionInput, string) (*studioBatchAsyncQueryOutput, error)

type studioBatchAsyncQueryOutput struct {
	Result        *AIImageAsyncResult
	Response      *StudioDesignResponse
	ResultPayload string
}

type studioBatchGenerationServiceConfig struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	submitAsync studioBatchGenerateAsyncSubmitter
	queryAsync  studioBatchGenerateAsyncQuerier
	currentTime func() time.Time
}

type studioBatchGenerationService struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	submitAsync studioBatchGenerateAsyncSubmitter
	queryAsync  studioBatchGenerateAsyncQuerier
	currentTime func() time.Time
}

func newStudioBatchGenerationService(config studioBatchGenerationServiceConfig) *studioBatchGenerationService {
	return &studioBatchGenerationService{
		repo:        config.repo,
		execute:     config.execute,
		submitAsync: config.submitAsync,
		queryAsync:  config.queryAsync,
		currentTime: config.currentTime,
	}
}

func (g *studioBatchGenerationService) RunPendingStudioBatchItems(ctx context.Context, batchID string) error {
	if g == nil || g.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}
	if g.execute == nil {
		return fmt.Errorf("studio batch execute function is not configured")
	}

	detail, err := g.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	for _, item := range detail.Items {
		if item.Status != StudioBatchItemStatusPending {
			continue
		}
		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusPending, StudioBatchItemStatusGenerating, g.now())
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			continue
		}
		attemptNo := len(detail.AttemptsByItem[item.ID]) + 1
		if err := g.runItemAttempt(ctx, detail.Batch, *claimedItem, attemptNo); err != nil {
			return err
		}
	}

	return g.refreshBatchStatus(ctx, detail.Batch.ID)
}

func (g *studioBatchGenerationService) RecoverStudioBatchMaterialization(ctx context.Context, batchID string) error {
	if g == nil || g.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}

	detail, err := g.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	for _, item := range detail.Items {
		attempts := detail.AttemptsByItem[item.ID]
		switch item.Status {
		case StudioBatchItemStatusAwaitingMaterialization:
			if err := g.recoverAwaitingMaterializationItem(ctx, detail.Batch, item, attempts); err != nil {
				return err
			}
		case StudioBatchItemStatusGenerating:
			if err := g.recoverGeneratingItem(ctx, detail.Batch, item, attempts); err != nil {
				return err
			}
		case StudioBatchItemStatusFailed:
			if err := g.recoverFailedItem(ctx, item, attempts); err != nil {
				return err
			}
		}
	}

	return g.refreshBatchStatus(ctx, detail.Batch.ID)
}

func (g *studioBatchGenerationService) runItemAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attemptNo int) error {
	request := buildStudioBatchItemDesignRequest(batch, item)
	requestPayload, err := json.Marshal(request)
	if err != nil {
		return err
	}

	nextAttemptNo := attemptNo
	for {
		now := g.now()
		attempt := &StudioGenerationAttemptRecord{
			ID:             buildStudioBatchAttemptID(item.ID, nextAttemptNo),
			ItemID:         item.ID,
			AttemptNo:      nextAttemptNo,
			Status:         StudioGenerationAttemptStatusQueued,
			RequestPayload: string(requestPayload),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := g.repo.CreateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}

		input := StudioBatchGenerateExecutionInput{
			BatchID:   batch.ID,
			ItemID:    item.ID,
			AttemptID: attempt.ID,
			Request:   request,
		}
		submitted, submitErr := g.submitAsyncItemAttempt(ctx, item, attempt, input)
		if submitErr != nil {
			finishedAt := g.now()
			attempt.Status = StudioGenerationAttemptStatusFailed
			attempt.ErrorMessage = submitErr.Error()
			attempt.FinishedAt = timePtr(finishedAt)
			attempt.UpdatedAt = finishedAt
			if updateErr := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); updateErr != nil {
				return updateErr
			}
			if shouldRetryStudioBatchAttempt(submitErr, nextAttemptNo) {
				nextAttemptNo++
				continue
			}
			item.Status = StudioBatchItemStatusFailed
			item.LastError = submitErr.Error()
			item.UpdatedAt = finishedAt
			return g.repo.UpdateStudioBatchItem(ctx, &item)
		}
		if submitted {
			return nil
		}

		execution, execErr := g.executeSyncItemAttempt(ctx, attempt, input)
		finishedAt := g.now()
		if execErr != nil {
			attempt.Status = StudioGenerationAttemptStatusFailed
			attempt.ErrorMessage = execErr.Error()
			attempt.FinishedAt = timePtr(finishedAt)
			attempt.UpdatedAt = finishedAt
			if updateErr := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); updateErr != nil {
				return updateErr
			}
			if shouldRetryStudioBatchAttempt(execErr, nextAttemptNo) {
				nextAttemptNo++
				continue
			}
			item.Status = StudioBatchItemStatusFailed
			item.LastError = execErr.Error()
			item.UpdatedAt = finishedAt
			return g.repo.UpdateStudioBatchItem(ctx, &item)
		}

		if err := g.finalizeSuccessfulAttempt(ctx, attempt, execution); err != nil {
			return err
		}

		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, finishedAt)
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			return nil
		}

		return g.materializeAttempt(ctx, batch, *claimedItem, attempt, execution.Response)
	}
}

func (g *studioBatchGenerationService) submitAsyncItemAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, input StudioBatchGenerateExecutionInput) (bool, error) {
	if g == nil || g.submitAsync == nil {
		return false, nil
	}
	submit, err := g.submitAsync(ctx, input)
	if err != nil {
		if errors.Is(err, ErrAsyncImageGenerationNotSupported) {
			return false, nil
		}
		return false, err
	}
	if submit == nil {
		return false, nil
	}
	now := g.now()
	attempt.Status = StudioGenerationAttemptStatusSubmitted
	attempt.Provider = strings.TrimSpace(submit.Provider)
	attempt.UpstreamJobID = strings.TrimSpace(submit.JobID)
	attempt.RequestID = strings.TrimSpace(submit.RequestID)
	attempt.SubmitResponsePayload = strings.TrimSpace(submit.RawSubmitResponse)
	attempt.StartedAt = timePtr(firstNonZeroStudioBatchTime(submit.AcceptedAt, now))
	attempt.UpdatedAt = now
	if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
		return false, err
	}

	item.Status = StudioBatchItemStatusGenerating
	item.LastError = ""
	item.UpdatedAt = now
	if err := g.repo.UpdateStudioBatchItem(ctx, &item); err != nil {
		return false, err
	}
	return true, nil
}

func (g *studioBatchGenerationService) executeSyncItemAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
	if g == nil || g.execute == nil {
		return nil, fmt.Errorf("studio batch execute function is not configured")
	}
	now := g.now()
	attempt.Status = StudioGenerationAttemptStatusRunning
	attempt.StartedAt = timePtr(now)
	attempt.UpdatedAt = now
	if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
		return nil, err
	}
	return g.execute(ctx, input)
}

func (g *studioBatchGenerationService) finalizeSuccessfulAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord, execution *StudioBatchGenerateExecutionOutput) error {
	finishedAt := g.now()
	attempt.Status = StudioGenerationAttemptStatusSucceeded
	attempt.FinishedAt = timePtr(finishedAt)
	attempt.UpdatedAt = finishedAt
	if execution != nil {
		attempt.ResultPayload = strings.TrimSpace(execution.ResultPayload)
		if execution.Response != nil {
			attempt.UpstreamJobID = strings.TrimSpace(execution.Response.UpstreamJobID)
			attempt.RequestID = firstNonEmpty(strings.TrimSpace(execution.Response.RequestID), attempt.RequestID)
		}
		if attempt.ResultPayload == "" && execution.Response != nil {
			payload, marshalErr := json.Marshal(execution.Response)
			if marshalErr != nil {
				return marshalErr
			}
			attempt.ResultPayload = string(payload)
		}
	}
	return g.repo.UpdateStudioGenerationAttempt(ctx, attempt)
}

func (g *studioBatchGenerationService) materializeAttempt(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, response *StudioDesignResponse) error {
	if response == nil || len(response.Images) == 0 {
		item.Status = StudioBatchItemStatusFailed
		item.LastError = "generation returned no images"
		item.UpdatedAt = g.now()
		return g.repo.UpdateStudioBatchItem(ctx, &item)
	}

	now := g.now()
	designs := make([]StudioMaterializedDesignRecord, 0, len(response.Images))
	for index, image := range response.Images {
		designID := strings.TrimSpace(image.ID)
		if designID == "" {
			designID = buildStudioBatchDesignID(attempt.ID, index)
		}
		designs = append(designs, StudioMaterializedDesignRecord{
			ID:               designID,
			BatchID:          item.BatchID,
			ItemID:           item.ID,
			SourceAttemptID:  attempt.ID,
			TargetGroupKey:   item.TargetGroupKey,
			TargetGroupLabel: item.TargetGroupLabel,
			ImageURL:         strings.TrimSpace(image.ImageURL),
			SortOrder:        index,
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}
	if err := g.repo.ReplaceStudioItemMaterializedDesigns(ctx, item.ID, designs); err != nil {
		return err
	}

	attempt.Status = StudioGenerationAttemptStatusMaterialized
	attempt.UpdatedAt = now
	if attempt.FinishedAt == nil {
		attempt.FinishedAt = timePtr(now)
	}
	if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
		return err
	}

	item.Status = StudioBatchItemStatusReviewReady
	item.LastError = ""
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func (g *studioBatchGenerationService) refreshBatchStatus(ctx context.Context, batchID string) error {
	detail, err := g.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}

	nextStatus := aggregateStudioBatchStatus(detail.Items)
	if detail.Batch.Status == nextStatus {
		return nil
	}
	batch := *detail.Batch
	batch.Status = nextStatus
	batch.UpdatedAt = g.now()
	return g.repo.UpdateStudioBatch(ctx, &batch)
}

func (g *studioBatchGenerationService) now() time.Time {
	if g != nil && g.currentTime != nil {
		return g.currentTime().UTC()
	}
	return time.Now().UTC()
}

func firstNonZeroStudioBatchTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
