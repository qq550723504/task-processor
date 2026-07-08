package sheinsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
			TenantID:       tenantID,
			StoreID:        storeID,
			ActivityType:   activityType,
			ActivityKey:    activityKey,
			ExecutableOnly: true,
			Page:           page,
			PageSize:       maxSheinEnrollmentCandidatePageSize,
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

func (s *sheinEnrollmentService) refreshCandidateCostOverrides(
	ctx context.Context,
	tenantID, storeID int64,
	candidates []SheinActivityCandidateRecord,
) ([]SheinActivityCandidateRecord, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	active := true
	products := make([]SheinSyncedProductRecord, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.SKCName == "" {
			continue
		}
		rows, _, err := s.repo.ListSyncedProducts(ctx, &SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			SKCName:  candidate.SKCName,
			IsActive: &active,
			Page:     1,
			PageSize: 1,
		})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}
		products = append(products, rows[0])
	}
	if len(products) == 0 {
		return candidates, nil
	}

	if groupReader, ok := s.repo.(sheinCandidateSDSCostGroupReader); ok {
		var err error
		products, err = applySheinSDSCostGroupOverrides(ctx, groupReader, tenantID, storeID, products)
		if err != nil {
			return nil, err
		}
	}
	productBySKC := make(map[string]SheinSyncedProductRecord, len(products))
	for _, product := range products {
		if product.SKCName == "" || product.EffectiveCostPrice == nil {
			continue
		}
		productBySKC[product.SKCName] = product
	}
	if len(productBySKC) == 0 {
		return candidates, nil
	}

	out := make([]SheinActivityCandidateRecord, len(candidates))
	copy(out, candidates)
	for i := range out {
		product, ok := productBySKC[out[i].SKCName]
		if !ok {
			continue
		}
		out[i].EffectiveCostPrice = cloneSheinSyncFloat64(product.EffectiveCostPrice)
		out[i].SKUCostPriceInfoList = cloneSheinSKUCostPriceList(product.SKUCostPriceInfoList)
		out[i].PriceSnapshot = refreshSheinEnrollmentPriceSnapshot(out[i].PriceSnapshot, product)
		out[i].CalculatedProfitRate = calculateSheinCandidateProfitRate(out[i].EffectiveCostPrice, out[i].PriceSnapshot)
	}
	return out, nil
}

func refreshSheinEnrollmentPriceSnapshot(existing string, product SheinSyncedProductRecord) string {
	if product.SupplyPrice == nil || *product.SupplyPrice <= 0 {
		return existing
	}

	payload := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &payload)
	}
	payload["sale_price"] = *product.SupplyPrice
	if items, ok := payload["sku_prices"].([]any); ok {
		for _, item := range items {
			if entry, ok := item.(map[string]any); ok {
				entry["sale_price"] = *product.SupplyPrice
			}
		}
	}
	if strings.TrimSpace(product.SupplyPriceCurrency) != "" {
		payload["currency"] = strings.TrimSpace(product.SupplyPriceCurrency)
	} else if strings.TrimSpace(product.Currency) != "" {
		payload["currency"] = strings.TrimSpace(product.Currency)
	} else if _, ok := payload["currency"]; !ok {
		payload["currency"] = product.Currency
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return existing
	}
	return string(encoded)
}

func filterExecutableSheinCandidates(
	candidates []SheinActivityCandidateRecord,
	triggerMode SheinEnrollmentRunTriggerMode,
) ([]SheinActivityCandidateRecord, map[int64]SheinActivityEnrollmentResult, map[int64]SheinActivityEnrollmentResult) {
	filtered := make([]SheinActivityCandidateRecord, 0, len(candidates))
	duplicateResults := make(map[int64]SheinActivityEnrollmentResult)
	nonExecutableResults := make(map[int64]SheinActivityEnrollmentResult)
	executableBySKC := make(map[string]int64)
	executableIndexBySKC := make(map[string]int)
	for _, candidate := range candidates {
		if reason := sheinCandidateNonExecutableReason(candidate, triggerMode); reason != "" {
			nonExecutableResults[candidate.ID] = SheinActivityEnrollmentResult{
				CandidateID:  candidate.ID,
				Success:      false,
				ErrorMessage: reason,
			}
			continue
		}
		if existingIndex, exists := executableIndexBySKC[candidate.SKCName]; exists {
			existing := filtered[existingIndex]
			if preferSheinCandidateForExecution(candidate, existing) {
				duplicateResults[existing.ID] = SheinActivityEnrollmentResult{
					CandidateID:  existing.ID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("duplicate executable candidate for SKC %s (already selected by candidate %d)", existing.SKCName, candidate.ID),
				}
				filtered[existingIndex] = candidate
				executableBySKC[candidate.SKCName] = candidate.ID
			} else {
				duplicateResults[candidate.ID] = SheinActivityEnrollmentResult{
					CandidateID:  candidate.ID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("duplicate executable candidate for SKC %s (already selected by candidate %d)", candidate.SKCName, executableBySKC[candidate.SKCName]),
				}
			}
			continue
		}
		executableBySKC[candidate.SKCName] = candidate.ID
		executableIndexBySKC[candidate.SKCName] = len(filtered)
		filtered = append(filtered, candidate)
	}
	return filtered, duplicateResults, nonExecutableResults
}

func preferSheinCandidateForExecution(candidate, existing SheinActivityCandidateRecord) bool {
	if candidate.SelectedForRun != existing.SelectedForRun {
		return candidate.SelectedForRun
	}
	if !candidate.UpdatedAt.Equal(existing.UpdatedAt) {
		return candidate.UpdatedAt.After(existing.UpdatedAt)
	}
	if !candidate.CreatedAt.Equal(existing.CreatedAt) {
		return candidate.CreatedAt.After(existing.CreatedAt)
	}
	return candidate.ID > existing.ID
}

func isExecutableSheinCandidate(candidate SheinActivityCandidateRecord, triggerMode SheinEnrollmentRunTriggerMode) bool {
	return sheinCandidateNonExecutableReason(candidate, triggerMode) == ""
}

func sheinCandidateNonExecutableReason(candidate SheinActivityCandidateRecord, triggerMode SheinEnrollmentRunTriggerMode) string {
	if candidate.EligibilityStatus != SheinCandidateEligibilityStatusEligible {
		return fmt.Sprintf(
			"candidate eligibility status %s is not executable for trigger mode %s",
			candidate.EligibilityStatus,
			triggerMode,
		)
	}
	switch candidate.ReviewStatus {
	case SheinCandidateReviewStatusApproved, SheinCandidateReviewStatusAutoQueued:
		return ""
	case SheinCandidateReviewStatusPendingReview:
		if triggerMode == SheinEnrollmentRunTriggerModeManualConfirmed {
			return ""
		}
		return fmt.Sprintf(
			"candidate review status %s is not executable for trigger mode %s",
			candidate.ReviewStatus,
			triggerMode,
		)
	default:
		return fmt.Sprintf(
			"candidate review status %s is not executable for trigger mode %s",
			candidate.ReviewStatus,
			triggerMode,
		)
	}
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
