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
	BaselineKey      string `json:"baseline_key,omitempty"`
	CacheStatus      string `json:"cache_status,omitempty"`
	ValidationStatus string `json:"validation_status,omitempty"`
	ReasonCode       string `json:"reason_code,omitempty"`
	Status           string `json:"status"`
	Reason           string `json:"reason,omitempty"`
}

const (
	SDSBaselineStatusMissing        = "missing"
	SDSBaselineStatusBaselineCached = "baseline_cached"
	SDSBaselineStatusReady          = "ready"
	SDSBaselineStatusBlocked        = "blocked"
	SDSBaselineStatusFailed         = "failed"
)

const (
	SDSBaselineValidationStatusUnknown = "unknown"
	SDSBaselineValidationStatusReady   = "ready"
	SDSBaselineValidationStatusBlocked = "blocked"
	SDSBaselineValidationStatusFailed  = "failed"
)

const (
	SDSBaselineReasonCodeMissingOptions             = "missing_options"
	SDSBaselineReasonCodeMissingParentProduct       = "missing_parent_product"
	SDSBaselineReasonCodeMissingPrototypeGroup      = "missing_prototype_group"
	SDSBaselineReasonCodeMissingVariant             = "missing_variant"
	SDSBaselineReasonCodeMissingDesignType          = "missing_design_type"
	SDSBaselineReasonCodeMissingPrintableSize       = "missing_printable_size"
	SDSBaselineReasonCodeMissingLayer               = "missing_layer"
	SDSBaselineReasonCodeLoginUnavailable           = "login_unavailable"
	SDSBaselineReasonCodeLoginInProgress            = "login_in_progress"
	SDSBaselineReasonCodeLoginMissingCredentials    = "login_missing_credentials"
	SDSBaselineReasonCodeLoginStatusCheckFailed     = "login_status_check_failed"
	SDSBaselineReasonCodeProductDetailCheckFailed   = "product_detail_check_failed"
	SDSBaselineReasonCodeProductDetailUnavailable   = "product_detail_unavailable"
	SDSBaselineReasonCodeDesignSurfaceCheckFailed   = "design_surface_check_failed"
	SDSBaselineReasonCodeDesignSurfaceUnavailable   = "design_surface_unavailable"
	SDSBaselineReasonCodeVariantMismatch            = "variant_mismatch"
	SDSBaselineReasonCodePrototypeGroupMismatch     = "prototype_group_mismatch"
	SDSBaselineReasonCodeLayerMissing               = "layer_missing"
	SDSBaselineReasonCodePrototypeGroupCheckFailed  = "prototype_group_check_failed"
	SDSBaselineReasonCodePrototypeGroupUnavailable  = "prototype_group_unavailable"
	SDSBaselineReasonCodeCacheRepositoryUnavailable = "cache_repository_unavailable"
	SDSBaselineReasonCodeCachePayloadMissing        = "cache_payload_missing"
	SDSBaselineReasonCodeCachePayloadInvalid        = "cache_payload_invalid"
	SDSBaselineReasonCodeCachePayloadEmpty          = "cache_payload_empty"
	SDSBaselineReasonCodeCacheUnavailable           = "cache_unavailable"
)

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
