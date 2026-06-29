package sheinsync

import (
	"context"
	"fmt"
	"time"
)

func (s *sheinEnrollmentService) ExecuteAutoSheinActivityEnrollment(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
) (*SheinActivityEnrollmentRunRecord, error) {
	if activityKey == "" {
		activityKey = buildSheinActivityKey(activityType, tenantID, storeID)
	}
	return s.ExecuteSheinActivityEnrollment(
		ctx,
		tenantID,
		storeID,
		activityType,
		activityKey,
		SheinEnrollmentRunTriggerModeAutoSchedule,
	)
}

func (s *sheinEnrollmentService) ExecuteSheinActivityEnrollment(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
	triggerMode SheinEnrollmentRunTriggerMode,
	candidateIDs ...int64,
) (*SheinActivityEnrollmentRunRecord, error) {
	run, candidates, err := s.prepareSheinActivityEnrollmentRun(ctx, tenantID, storeID, activityType, activityKey, triggerMode, candidateIDs...)
	if err != nil {
		return nil, err
	}
	return s.completeSheinActivityEnrollmentRun(ctx, run, candidates)
}

func (s *sheinEnrollmentService) StartSheinActivityEnrollment(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
	triggerMode SheinEnrollmentRunTriggerMode,
	candidateIDs ...int64,
) (*SheinActivityEnrollmentRunRecord, error) {
	run, candidates, err := s.prepareSheinActivityEnrollmentRun(ctx, tenantID, storeID, activityType, activityKey, triggerMode, candidateIDs...)
	if err != nil {
		return nil, err
	}
	runSnapshot := *run
	candidateSnapshot := cloneSheinEnrollmentCandidateRecords(candidates)
	go func() {
		_, _ = s.completeSheinActivityEnrollmentRun(context.WithoutCancel(ctx), &runSnapshot, candidateSnapshot)
	}()
	return run, nil
}

func (s *sheinEnrollmentService) prepareSheinActivityEnrollmentRun(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
	triggerMode SheinEnrollmentRunTriggerMode,
	candidateIDs ...int64,
) (*SheinActivityEnrollmentRunRecord, []SheinActivityCandidateRecord, error) {
	if err := s.validate(); err != nil {
		return nil, nil, err
	}
	if activityType == "" {
		return nil, nil, fmt.Errorf("SHEIN enrollment activity type is required")
	}
	if activityKey == "" {
		activityKey = buildSheinActivityKey(activityType, tenantID, storeID)
	}

	candidates, err := s.listCandidates(ctx, tenantID, storeID, activityType, activityKey, candidateIDs)
	if err != nil {
		return nil, nil, err
	}
	candidates, err = s.refreshCandidateCostOverrides(ctx, tenantID, storeID, candidates)
	if err != nil {
		return nil, nil, err
	}

	startedAt := time.Now().UTC()
	run := &SheinActivityEnrollmentRunRecord{
		TenantID:       tenantID,
		StoreID:        storeID,
		ActivityType:   activityType,
		ActivityKey:    activityKey,
		TriggerMode:    triggerMode,
		Status:         SheinEnrollmentRunStatusRunning,
		CandidateCount: len(candidates),
		StartedAt:      &startedAt,
	}
	if err := s.repo.CreateEnrollmentRun(ctx, run); err != nil {
		return nil, nil, err
	}

	return run, candidates, nil
}

func (s *sheinEnrollmentService) completeSheinActivityEnrollmentRun(
	ctx context.Context,
	run *SheinActivityEnrollmentRunRecord,
	candidates []SheinActivityCandidateRecord,
) (*SheinActivityEnrollmentRunRecord, error) {
	executable, duplicateResults, nonExecutableResults := filterExecutableSheinCandidates(candidates, run.TriggerMode)
	results, adapterErr := s.executeCandidates(ctx, run.StoreID, run.ActivityType, run.ActivityKey, executable)
	candidateResultByID := mapSheinEnrollmentResults(executable, results, adapterErr)
	for candidateID, result := range duplicateResults {
		candidateResultByID[candidateID] = result
	}
	itemResultByID := cloneSheinEnrollmentResultMap(candidateResultByID)
	for candidateID, result := range nonExecutableResults {
		itemResultByID[candidateID] = result
	}
	items := buildSheinEnrollmentItems(run.ID, run.StoreID, candidates, itemResultByID)
	mutatedCandidates := buildSheinEnrollmentCandidateUpdates(candidates, candidateResultByID)

	run.SubmittedCount = len(executable)
	run.SucceededCount, run.FailedCount = countSheinEnrollmentOutcomes(itemResultByID)
	run.Status = deriveSheinEnrollmentRunStatus(run.CandidateCount, run.SubmittedCount, run.SucceededCount, run.FailedCount, adapterErr)
	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	if adapterErr != nil {
		run.ErrorSummary = adapterErr.Error()
	} else if run.CandidateCount > 0 && run.SubmittedCount == 0 {
		run.ErrorSummary = "no executable SHEIN enrollment candidates"
	}

	persistErr := s.persistEnrollmentOutcome(ctx, run, items, mutatedCandidates)
	returnErr := joinSheinEnrollmentErrors(adapterErr, persistErr)
	if returnErr != nil {
		return run, returnErr
	}
	return run, nil
}

func (s *sheinEnrollmentService) validate() error {
	switch {
	case s == nil:
		return fmt.Errorf("SHEIN enrollment service is required")
	case s.repo == nil:
		return fmt.Errorf("SHEIN enrollment repository is required")
	case s.adapter == nil:
		return fmt.Errorf("SHEIN activity adapter is required")
	default:
		return nil
	}
}

func cloneSheinEnrollmentCandidateRecords(source []SheinActivityCandidateRecord) []SheinActivityCandidateRecord {
	if len(source) == 0 {
		return nil
	}
	copied := make([]SheinActivityCandidateRecord, len(source))
	copy(copied, source)
	return copied
}

func (s *sheinEnrollmentService) executeCandidates(
	ctx context.Context,
	storeID int64,
	activityType string,
	activityKey string,
	candidates []SheinActivityCandidateRecord,
) ([]SheinActivityEnrollmentResult, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	payload := make([]SheinActivityEnrollmentCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		payload = append(payload, SheinActivityEnrollmentCandidate{
			CandidateID:          candidate.ID,
			SyncedProductID:      candidate.SyncedProductID,
			ActivityKey:          candidate.ActivityKey,
			CandidateVersion:     candidate.CandidateVersion,
			SKCName:              candidate.SKCName,
			EffectiveCostPrice:   cloneSheinEnrollmentFloat64(candidate.EffectiveCostPrice),
			PriceSnapshot:        candidate.PriceSnapshot,
			InventorySnapshot:    candidate.InventorySnapshot,
			CalculatedProfitRate: cloneSheinEnrollmentFloat64(candidate.CalculatedProfitRate),
		})
	}
	return s.adapter.EnrollCandidates(ctx, storeID, activityType, activityKey, payload)
}

func ensureSheinEnrollmentSingleCandidateResult(
	candidates []SheinActivityEnrollmentCandidate,
	results []SheinActivityEnrollmentResult,
	err error,
) []SheinActivityEnrollmentResult {
	if len(candidates) != 1 || err == nil {
		return results
	}
	candidateID := candidates[0].CandidateID
	for _, result := range results {
		if result.CandidateID == candidateID {
			return results
		}
	}
	return append(results, SheinActivityEnrollmentResult{
		CandidateID:  candidateID,
		Success:      false,
		ErrorMessage: err.Error(),
	})
}
