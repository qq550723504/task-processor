package sheinsync

import (
	"context"
	"fmt"
	"sort"
)

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
