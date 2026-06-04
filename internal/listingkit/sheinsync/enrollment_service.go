package sheinsync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type SheinEnrollmentService interface {
	ExecuteSheinActivityEnrollment(
		ctx context.Context,
		tenantID, storeID int64,
		activityType string,
		activityKey string,
		triggerMode SheinEnrollmentRunTriggerMode,
		candidateIDs ...int64,
	) (*SheinActivityEnrollmentRunRecord, error)
	ExecuteAutoSheinActivityEnrollment(ctx context.Context, tenantID, storeID int64, activityType string, activityKey string) (*SheinActivityEnrollmentRunRecord, error)
}

type SheinEnrollmentRepository interface {
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
	SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error
	CreateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	UpdateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	SaveEnrollmentItems(ctx context.Context, items []*SheinActivityEnrollmentItemRecord) error
}

type sheinEnrollmentService struct {
	repo    SheinEnrollmentRepository
	adapter SheinActivityAdapter
}

func NewSheinEnrollmentService(repo SheinEnrollmentRepository, adapter SheinActivityAdapter) SheinEnrollmentService {
	return &sheinEnrollmentService{
		repo:    repo,
		adapter: adapter,
	}
}

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

	executable, duplicateResults := filterExecutableSheinCandidates(candidates)
	results, adapterErr := s.executeCandidates(ctx, storeID, activityType, activityKey, executable)
	resultByCandidateID := mapSheinEnrollmentResults(executable, results, adapterErr)
	for candidateID, result := range duplicateResults {
		resultByCandidateID[candidateID] = result
	}
	items := buildSheinEnrollmentItems(run.ID, storeID, candidates, resultByCandidateID)
	mutatedCandidates := buildSheinEnrollmentCandidateUpdates(candidates, resultByCandidateID)

	run.SubmittedCount = len(executable)
	run.SucceededCount, run.FailedCount = countSheinEnrollmentOutcomes(resultByCandidateID)
	run.Status = deriveSheinEnrollmentRunStatus(run.SubmittedCount, run.SucceededCount, run.FailedCount, adapterErr)
	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	if adapterErr != nil {
		run.ErrorSummary = adapterErr.Error()
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

func (s *sheinEnrollmentService) listCandidates(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
	candidateIDs []int64,
) ([]SheinActivityCandidateRecord, error) {
	uniqueCandidateIDs := uniqueSheinEnrollmentIDs(candidateIDs)
	if len(uniqueCandidateIDs) > 0 {
		return s.listCandidatesByIDs(ctx, tenantID, storeID, activityType, activityKey, uniqueCandidateIDs)
	}
	return s.listCandidatesByPage(ctx, tenantID, storeID, activityType, activityKey)
}

func (s *sheinEnrollmentService) listCandidatesByIDs(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
	candidateIDs []int64,
) ([]SheinActivityCandidateRecord, error) {
	rows, _, err := s.repo.ListCandidates(ctx, &SheinActivityCandidateQuery{
		TenantID:     tenantID,
		StoreID:      storeID,
		ActivityType: activityType,
		ActivityKey:  activityKey,
		CandidateIDs: append([]int64(nil), candidateIDs...),
		Page:         1,
		PageSize:     len(candidateIDs),
	})
	if err != nil {
		return nil, err
	}

	found := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		found[row.ID] = struct{}{}
	}
	missing := make([]int64, 0)
	for _, id := range candidateIDs {
		if _, ok := found[id]; ok {
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		return nil, fmt.Errorf("missing SHEIN enrollment candidates: %v", missing)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func (s *sheinEnrollmentService) listCandidatesByPage(
	ctx context.Context,
	tenantID, storeID int64,
	activityType string,
	activityKey string,
) ([]SheinActivityCandidateRecord, error) {
	page := 1
	items := make([]SheinActivityCandidateRecord, 0)
	for {
		rows, total, err := s.repo.ListCandidates(ctx, &SheinActivityCandidateQuery{
			TenantID:     tenantID,
			StoreID:      storeID,
			ActivityType: activityType,
			ActivityKey:  activityKey,
			Page:         page,
			PageSize:     maxSheinEnrollmentCandidatePageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
		if len(rows) == 0 || int64(page*maxSheinEnrollmentCandidatePageSize) >= total {
			break
		}
		page++
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
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

const maxSheinEnrollmentCandidatePageSize = 200

func (s *sheinEnrollmentService) persistEnrollmentOutcome(
	ctx context.Context,
	run *SheinActivityEnrollmentRunRecord,
	items []*SheinActivityEnrollmentItemRecord,
	candidates []*SheinActivityCandidateRecord,
) error {
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

func filterExecutableSheinCandidates(candidates []SheinActivityCandidateRecord) ([]SheinActivityCandidateRecord, map[int64]SheinActivityEnrollmentResult) {
	filtered := make([]SheinActivityCandidateRecord, 0, len(candidates))
	duplicateResults := make(map[int64]SheinActivityEnrollmentResult)
	executableBySKC := make(map[string]int64)
	for _, candidate := range candidates {
		if candidate.ReviewStatus != SheinCandidateReviewStatusApproved &&
			candidate.ReviewStatus != SheinCandidateReviewStatusAutoQueued {
			continue
		}
		if existingCandidateID, exists := executableBySKC[candidate.SKCName]; exists {
			duplicateResults[candidate.ID] = SheinActivityEnrollmentResult{
				CandidateID:  candidate.ID,
				Success:      false,
				ErrorMessage: fmt.Sprintf("duplicate executable candidate for SKC %s (already selected by candidate %d)", candidate.SKCName, existingCandidateID),
			}
			continue
		}
		executableBySKC[candidate.SKCName] = candidate.ID
		filtered = append(filtered, candidate)
	}
	return filtered, duplicateResults
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

func deriveSheinEnrollmentRunStatus(submittedCount, succeededCount, failedCount int, adapterErr error) SheinEnrollmentRunStatus {
	switch {
	case submittedCount == 0:
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

func uniqueSheinEnrollmentIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
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
