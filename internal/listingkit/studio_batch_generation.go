package listingkit

import (
	"context"
	"encoding/json"
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

type studioBatchGenerationServiceConfig struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	currentTime func() time.Time
}

type studioBatchGenerationService struct {
	repo        StudioBatchRepository
	execute     studioBatchGenerateExecutor
	currentTime func() time.Time
}

func newStudioBatchGenerationService(config studioBatchGenerationServiceConfig) *studioBatchGenerationService {
	return &studioBatchGenerationService{
		repo:        config.repo,
		execute:     config.execute,
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
			Status:         StudioGenerationAttemptStatusRunning,
			RequestPayload: string(requestPayload),
			StartedAt:      timePtr(now),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := g.repo.CreateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}

		execution, execErr := g.execute(ctx, StudioBatchGenerateExecutionInput{
			BatchID:   batch.ID,
			ItemID:    item.ID,
			AttemptID: attempt.ID,
			Request:   request,
		})
		finishedAt := g.now()
		attempt.FinishedAt = timePtr(finishedAt)
		attempt.UpdatedAt = finishedAt
		if execErr != nil {
			attempt.Status = StudioGenerationAttemptStatusFailed
			attempt.ErrorMessage = execErr.Error()
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

		attempt.Status = StudioGenerationAttemptStatusSucceeded
		attempt.ResultPayload = strings.TrimSpace(execution.ResultPayload)
		if execution.Response != nil {
			attempt.UpstreamJobID = strings.TrimSpace(execution.Response.UpstreamJobID)
		}
		if attempt.ResultPayload == "" && execution.Response != nil {
			payload, marshalErr := json.Marshal(execution.Response)
			if marshalErr != nil {
				return marshalErr
			}
			attempt.ResultPayload = string(payload)
		}
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
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
