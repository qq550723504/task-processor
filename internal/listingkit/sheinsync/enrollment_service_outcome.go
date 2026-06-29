package sheinsync

import (
	"context"
	"errors"
	"fmt"
)

func (s *sheinEnrollmentService) persistEnrollmentOutcome(
	ctx context.Context,
	run *SheinActivityEnrollmentRunRecord,
	items []*SheinActivityEnrollmentItemRecord,
	candidates []*SheinActivityCandidateRecord,
) error {
	ctx = context.WithoutCancel(ctx)
	initialRunUpdateErr := s.bestEffortUpdateEnrollmentRun(ctx, run)

	persistErrs := make([]error, 0, 2)
	if len(items) > 0 {
		if err := s.repo.SaveEnrollmentItems(ctx, items); err != nil {
			persistErrs = append(persistErrs, fmt.Errorf("persist SHEIN enrollment items: %w", err))
		}
	}
	if len(candidates) > 0 {
		if err := s.repo.SaveCandidates(ctx, candidates); err != nil {
			persistErrs = append(persistErrs, fmt.Errorf("persist SHEIN enrollment candidates: %w", err))
		}
	}
	if len(persistErrs) > 0 {
		run.Status = SheinEnrollmentRunStatusFailed
		run.ErrorSummary = joinSheinEnrollmentSummary(run.ErrorSummary, joinSheinEnrollmentErrors(persistErrs...).Error())
	}

	if len(persistErrs) == 0 && initialRunUpdateErr == nil {
		return nil
	}
	finalRunUpdateErr := s.bestEffortUpdateEnrollmentRun(ctx, run)
	return joinSheinEnrollmentErrors(initialRunUpdateErr, joinSheinEnrollmentErrors(persistErrs...), finalRunUpdateErr)
}

func (s *sheinEnrollmentService) bestEffortUpdateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error {
	if err := s.repo.UpdateEnrollmentRun(ctx, run); err != nil {
		return fmt.Errorf("persist SHEIN enrollment run: %w", err)
	}
	return nil
}

func mapSheinEnrollmentResults(
	candidates []SheinActivityCandidateRecord,
	results []SheinActivityEnrollmentResult,
	adapterErr error,
) map[int64]SheinActivityEnrollmentResult {
	mapped := make(map[int64]SheinActivityEnrollmentResult, len(candidates))
	for _, result := range results {
		mapped[result.CandidateID] = result
	}
	for _, candidate := range candidates {
		if _, ok := mapped[candidate.ID]; ok {
			continue
		}
		fallback := SheinActivityEnrollmentResult{
			CandidateID: candidate.ID,
			Success:     false,
		}
		if adapterErr != nil {
			fallback.ErrorMessage = adapterErr.Error()
		} else {
			fallback.ErrorMessage = "missing SHEIN enrollment result"
		}
		mapped[candidate.ID] = fallback
	}
	return mapped
}

func cloneSheinEnrollmentResultMap(source map[int64]SheinActivityEnrollmentResult) map[int64]SheinActivityEnrollmentResult {
	copied := make(map[int64]SheinActivityEnrollmentResult, len(source))
	for candidateID, result := range source {
		copied[candidateID] = result
	}
	return copied
}

func buildSheinEnrollmentItems(
	runID, storeID int64,
	candidates []SheinActivityCandidateRecord,
	resultByCandidateID map[int64]SheinActivityEnrollmentResult,
) []*SheinActivityEnrollmentItemRecord {
	items := make([]*SheinActivityEnrollmentItemRecord, 0, len(candidates))
	for _, candidate := range candidates {
		result, ok := resultByCandidateID[candidate.ID]
		if !ok {
			continue
		}
		status := SheinEnrollmentItemStatusFailed
		if result.Success {
			status = SheinEnrollmentItemStatusSucceeded
		}
		items = append(items, &SheinActivityEnrollmentItemRecord{
			RunID:            runID,
			CandidateID:      candidate.ID,
			StoreID:          storeID,
			ActivityKey:      candidate.ActivityKey,
			CandidateVersion: candidate.CandidateVersion,
			SyncedProductID:  candidate.SyncedProductID,
			SKCName:          candidate.SKCName,
			Status:           status,
			RequestPayload:   result.RequestPayload,
			ResponsePayload:  result.ResponsePayload,
			ErrorMessage:     result.ErrorMessage,
		})
	}
	return items
}

func buildSheinEnrollmentCandidateUpdates(
	candidates []SheinActivityCandidateRecord,
	resultByCandidateID map[int64]SheinActivityEnrollmentResult,
) []*SheinActivityCandidateRecord {
	updates := make([]*SheinActivityCandidateRecord, 0, len(candidates))
	for _, candidate := range candidates {
		result, ok := resultByCandidateID[candidate.ID]
		if !ok {
			continue
		}
		row := candidate
		if result.Success {
			row.ReviewStatus = SheinCandidateReviewStatusEnrolled
		} else {
			row.ReviewStatus = SheinCandidateReviewStatusFailed
		}
		updates = append(updates, &row)
	}
	return updates
}

func countSheinEnrollmentOutcomes(resultByCandidateID map[int64]SheinActivityEnrollmentResult) (int, int) {
	succeeded := 0
	failed := 0
	for _, result := range resultByCandidateID {
		if result.Success {
			succeeded++
			continue
		}
		failed++
	}
	return succeeded, failed
}

func deriveSheinEnrollmentRunStatus(candidateCount, submittedCount, succeededCount, failedCount int, adapterErr error) SheinEnrollmentRunStatus {
	switch {
	case submittedCount == 0:
		if candidateCount > 0 && failedCount == 0 {
			return SheinEnrollmentRunStatusFailed
		}
		if failedCount > 0 {
			return SheinEnrollmentRunStatusFailed
		}
		return SheinEnrollmentRunStatusSucceeded
	case succeededCount > 0 && failedCount > 0:
		return SheinEnrollmentRunStatusPartiallySucceeded
	case failedCount > 0:
		return SheinEnrollmentRunStatusFailed
	case adapterErr != nil:
		return SheinEnrollmentRunStatusFailed
	default:
		return SheinEnrollmentRunStatusSucceeded
	}
}

func cloneSheinEnrollmentFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func joinSheinEnrollmentErrors(errs ...error) error {
	joined := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			joined = append(joined, err)
		}
	}
	if len(joined) == 0 {
		return nil
	}
	return errors.Join(joined...)
}

func joinSheinEnrollmentSummary(current string, next string) string {
	switch {
	case current == "":
		return next
	case next == "":
		return current
	default:
		return current + "; " + next
	}
}
