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
	if err := s.validate(); err != nil {
		return nil, err
	}
	if activityType == "" {
		return nil, fmt.Errorf("SHEIN enrollment activity type is required")
	}
	if activityKey == "" {
		activityKey = buildSheinActivityKey(activityType, tenantID, storeID)
	}

	candidates, err := s.listCandidates(ctx, tenantID, storeID, activityType, activityKey, candidateIDs)
	if err != nil {
		return nil, err
	}
	candidates, err = s.refreshCandidateCostOverrides(ctx, tenantID, storeID, candidates)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	executable, duplicateResults, nonExecutableResults := filterExecutableSheinCandidates(candidates, triggerMode)
	results, adapterErr := s.executeCandidates(ctx, storeID, activityType, activityKey, executable)
	candidateResultByID := mapSheinEnrollmentResults(executable, results, adapterErr)
	for candidateID, result := range duplicateResults {
		candidateResultByID[candidateID] = result
	}
	itemResultByID := cloneSheinEnrollmentResultMap(candidateResultByID)
	for candidateID, result := range nonExecutableResults {
		itemResultByID[candidateID] = result
	}
	items := buildSheinEnrollmentItems(run.ID, storeID, candidates, itemResultByID)
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
