package listingkit

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (g *studioBatchGenerationService) recoverAwaitingMaterializationItem(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestRecoverableStudioBatchAttempt(attempts)
	if attempt == nil {
		return g.failItemAndAttempt(ctx, item, nil, "materialization recovery missing generation attempt")
	}
	if strings.TrimSpace(attempt.ResultPayload) == "" {
		return g.failItemAndAttempt(ctx, item, attempt, "generation result payload missing")
	}

	claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusAwaitingMaterialization, StudioBatchItemStatusGenerating, g.now())
	if err != nil {
		return err
	}
	if !claimed || claimedItem == nil {
		return nil
	}

	var response StudioDesignResponse
	if err := json.Unmarshal([]byte(attempt.ResultPayload), &response); err != nil {
		return g.failItemAndAttempt(ctx, *claimedItem, attempt, "generation result payload invalid")
	}
	return g.materializeAttempt(ctx, batch, *claimedItem, attempt, &response)
}

func (g *studioBatchGenerationService) recoverGeneratingItem(ctx context.Context, batch *StudioBatchRecord, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestStudioBatchAttempt(attempts)
	if attempt == nil {
		return g.failItemAndAttempt(ctx, item, nil, "generation interrupted before attempt persisted")
	}

	switch attempt.Status {
	case StudioGenerationAttemptStatusSucceeded, StudioGenerationAttemptStatusMaterialized:
		if strings.TrimSpace(attempt.ResultPayload) == "" {
			return g.failItemAndAttempt(ctx, item, attempt, "generation result payload missing")
		}
		claimedItem, claimed, err := g.repo.ClaimStudioBatchItem(ctx, item.ID, StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, g.now())
		if err != nil {
			return err
		}
		if !claimed || claimedItem == nil {
			return nil
		}
		var response StudioDesignResponse
		if err := json.Unmarshal([]byte(attempt.ResultPayload), &response); err != nil {
			return g.failItemAndAttempt(ctx, *claimedItem, attempt, "generation result payload invalid")
		}
		return g.recoverAwaitingMaterializationItem(ctx, batch, *claimedItem, attempts)
	case StudioGenerationAttemptStatusRunning, StudioGenerationAttemptStatusQueued:
		if !isStudioBatchAttemptStale(attempt, g.now()) {
			return nil
		}
		message := "generation attempt timed out before result persisted"
		if shouldRetryStudioBatchRecoveredFailure(message, attempt.AttemptNo) {
			return g.requeueItemAfterFailedAttempt(ctx, item, attempt, message)
		}
		return g.failItemAndAttempt(ctx, item, attempt, message)
	default:
		return g.failItemAndAttempt(ctx, item, attempt, firstNonEmpty(strings.TrimSpace(attempt.ErrorMessage), "generation failed"))
	}
}

func (g *studioBatchGenerationService) recoverFailedItem(ctx context.Context, item StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord) error {
	attempt := latestStudioBatchAttempt(attempts)
	if attempt == nil {
		return nil
	}
	if attempt.Status != StudioGenerationAttemptStatusFailed {
		return nil
	}
	message := firstNonEmpty(strings.TrimSpace(attempt.ErrorMessage), strings.TrimSpace(item.LastError))
	if !shouldRetryStudioBatchRecoveredFailure(message, attempt.AttemptNo) {
		return nil
	}
	return g.requeueItemAfterFailedAttempt(ctx, item, attempt, message)
}

func isStudioBatchAttemptStale(attempt *StudioGenerationAttemptRecord, now time.Time) bool {
	if attempt == nil {
		return false
	}
	referenceTime := attempt.UpdatedAt
	if attempt.StartedAt != nil && attempt.StartedAt.After(referenceTime) {
		referenceTime = *attempt.StartedAt
	}
	if referenceTime.IsZero() {
		referenceTime = attempt.CreatedAt
	}
	if referenceTime.IsZero() {
		return false
	}
	return now.UTC().Sub(referenceTime.UTC()) >= defaultStudioBatchAttemptStaleAfter
}

func latestStudioBatchAttempt(attempts []StudioGenerationAttemptRecord) *StudioGenerationAttemptRecord {
	if len(attempts) == 0 {
		return nil
	}
	cloned := attempts[len(attempts)-1]
	return &cloned
}

func (g *studioBatchGenerationService) failItemAndAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, message string) error {
	now := g.now()
	if attempt != nil {
		attempt.Status = StudioGenerationAttemptStatusFailed
		attempt.ErrorMessage = message
		if attempt.FinishedAt == nil {
			attempt.FinishedAt = timePtr(now)
		}
		attempt.UpdatedAt = now
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}
	}

	item.Status = StudioBatchItemStatusFailed
	item.LastError = message
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func (g *studioBatchGenerationService) requeueItemAfterFailedAttempt(ctx context.Context, item StudioBatchItemRecord, attempt *StudioGenerationAttemptRecord, message string) error {
	now := g.now()
	if attempt != nil {
		attempt.Status = StudioGenerationAttemptStatusFailed
		attempt.ErrorMessage = message
		if attempt.FinishedAt == nil {
			attempt.FinishedAt = timePtr(now)
		}
		attempt.UpdatedAt = now
		if err := g.repo.UpdateStudioGenerationAttempt(ctx, attempt); err != nil {
			return err
		}
	}

	item.Status = StudioBatchItemStatusPending
	item.LastError = ""
	item.UpdatedAt = now
	return g.repo.UpdateStudioBatchItem(ctx, &item)
}

func shouldRetryStudioBatchAttempt(err error, attemptNo int) bool {
	if err == nil {
		return false
	}
	return shouldRetryStudioBatchAttemptMessage(err.Error(), attemptNo)
}

func shouldRetryStudioBatchRecoveredFailure(message string, attemptNo int) bool {
	if isStudioBatchTimeoutFailureMessage(message) {
		return attemptNo < defaultStudioBatchStaleRecoveryLimit
	}
	return shouldRetryStudioBatchAttemptMessage(message, attemptNo)
}

func shouldRetryStudioBatchAttemptMessage(message string, attemptNo int) bool {
	if attemptNo >= defaultStudioBatchTransientRetryLimit {
		return false
	}
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	return isStudioBatchTransientRetryMessage(message)
}

func isStudioBatchTimeoutFailureMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "timed out") ||
		strings.Contains(message, "gateway timeout")
}

func isStudioBatchTransientRetryMessage(message string) bool {
	if message == "" {
		return false
	}
	return strings.Contains(message, "excessive system load") ||
		strings.Contains(message, "rate limit") ||
		strings.Contains(message, "rate limited") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "timed out") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "service unavailable") ||
		strings.Contains(message, "bad gateway") ||
		strings.Contains(message, "gateway timeout")
}

func latestRecoverableStudioBatchAttempt(attempts []StudioGenerationAttemptRecord) *StudioGenerationAttemptRecord {
	for index := len(attempts) - 1; index >= 0; index-- {
		attempt := attempts[index]
		if attempt.Status != StudioGenerationAttemptStatusSucceeded && attempt.Status != StudioGenerationAttemptStatusMaterialized {
			continue
		}
		cloned := attempt
		return &cloned
	}
	return nil
}
