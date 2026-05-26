package listingkit

import (
	"context"
	"fmt"
	"strings"
)

type SDSBaselineReadinessQuery struct {
	TenantID           string  `json:"tenant_id,omitempty"`
	ParentProductID    int64   `json:"parent_product_id,omitempty"`
	PrototypeGroupID   int64   `json:"prototype_group_id,omitempty"`
	VariantID          int64   `json:"variant_id,omitempty"`
	SelectedVariantIDs []int64 `json:"selected_variant_ids,omitempty"`
}

type SDSBaselineReadiness struct {
	BaselineKey string `json:"baseline_key,omitempty"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
}

func (q *SDSBaselineReadinessQuery) Validate() error {
	if q == nil {
		return fmt.Errorf("query cannot be nil")
	}
	if q.ParentProductID <= 0 {
		return fmt.Errorf("parent_product_id must be positive")
	}
	if q.PrototypeGroupID <= 0 {
		return fmt.Errorf("prototype_group_id must be positive")
	}
	if q.VariantID <= 0 {
		return fmt.Errorf("variant_id must be positive")
	}
	return nil
}

func (q *SDSBaselineReadinessQuery) BaselineOptions() *SDSSyncOptions {
	if q == nil {
		return nil
	}
	return &SDSSyncOptions{
		ParentProductID:  q.ParentProductID,
		PrototypeGroupID: q.PrototypeGroupID,
		VariantID:        q.VariantID,
		Variants:         baselineVariantsFromIDs(q.SelectedVariantIDs),
	}
}

func resolveSDSBaselineReadinessTenant(ctx context.Context, tenantID string) string {
	if trimmed := strings.TrimSpace(tenantID); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(TenantIDFromContext(ctx))
}

func baselineVariantsFromIDs(ids []int64) []SDSSyncVariantOption {
	if len(ids) == 0 {
		return nil
	}
	result := make([]SDSSyncVariantOption, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		result = append(result, SDSSyncVariantOption{VariantID: id})
	}
	return result
}
