package listingkit

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit/tenantctx"
)

const sdsBaselineCacheKeyVersion = "listingkit:sds_baseline:v1:"

var ErrSDSBaselineTenantMismatch = errors.New("sds baseline tenant does not match context tenant")

type SDSBaselineIdentity struct {
	ParentProductID    int64   `json:"parent_product_id,omitempty"`
	PrototypeGroupID   int64   `json:"prototype_group_id,omitempty"`
	VariantID          int64   `json:"variant_id,omitempty"`
	SelectedVariantIDs []int64 `json:"selected_variant_ids,omitempty"`
}

func (i SDSBaselineIdentity) Value() (driver.Value, error) {
	return json.Marshal(i)
}

func (i *SDSBaselineIdentity) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, i)
}

type SDSBaselineCacheEntry struct {
	TenantID             string                        `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	BaselineKey          string                        `json:"baseline_key" gorm:"primaryKey;type:varchar(128)"`
	Status               string                        `json:"status,omitempty" gorm:"type:varchar(20);index"`
	Version              int                           `json:"version"`
	SourceTaskID         string                        `json:"source_task_id,omitempty" gorm:"type:varchar(36);index"`
	Identity             SDSBaselineIdentity           `json:"identity" gorm:"type:text"`
	CanonicalProductBase *CanonicalProductCachePayload `json:"canonical_product_base,omitempty" gorm:"type:text"`
	ValidationStatus     string                        `json:"validation_status,omitempty" gorm:"type:varchar(20);index"`
	ValidationReasonCode string                        `json:"validation_reason_code,omitempty" gorm:"type:varchar(64);index"`
	ValidationReason     string                        `json:"validation_reason,omitempty" gorm:"type:text"`
	ValidatedAt          *time.Time                    `json:"validated_at,omitempty"`
	CreatedAt            time.Time                     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time                     `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SDSBaselineCacheEntry) TableName() string {
	return "listing_kit_sds_baseline_cache"
}

func (e *SDSBaselineCacheEntry) CanonicalProduct() (*canonical.Product, error) {
	if e == nil || e.CanonicalProductBase == nil {
		return nil, nil
	}
	return e.CanonicalProductBase.CanonicalProduct()
}

func (e *SDSBaselineCacheEntry) Clone() (*SDSBaselineCacheEntry, error) {
	if e == nil {
		return nil, nil
	}
	cloned := *e
	cloned.Identity.SelectedVariantIDs = append([]int64(nil), e.Identity.SelectedVariantIDs...)
	if e.CanonicalProductBase != nil {
		product, err := e.CanonicalProductBase.CanonicalProduct()
		if err != nil {
			return nil, err
		}
		payload, err := newCanonicalProductCachePayload(product)
		if err != nil {
			return nil, err
		}
		cloned.CanonicalProductBase = payload
	}
	return &cloned, nil
}

func SDSBaselineKeyFromOptions(tenantID string, options *SDSSyncOptions) string {
	return sdsBaselineKey(tenantID, options)
}

func sdsBaselineKey(tenantID string, options *SDSSyncOptions) string {
	identity := sdsBaselineIdentityFromOptions(options)
	if isEmptySDSBaselineIdentity(identity) {
		return ""
	}
	source := struct {
		TenantID string              `json:"tenant_id,omitempty"`
		Identity SDSBaselineIdentity `json:"identity"`
	}{
		TenantID: strings.TrimSpace(tenantID),
		Identity: identity,
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%s%x", sdsBaselineCacheKeyVersion, sum[:])
}

func isEmptySDSBaselineIdentity(identity SDSBaselineIdentity) bool {
	return identity.ParentProductID == 0 &&
		identity.PrototypeGroupID == 0 &&
		identity.VariantID == 0 &&
		len(identity.SelectedVariantIDs) == 0
}

func sdsBaselineIdentityFromOptions(options *SDSSyncOptions) SDSBaselineIdentity {
	if options == nil {
		return SDSBaselineIdentity{}
	}
	identity := SDSBaselineIdentity{
		ParentProductID:  options.ParentProductID,
		PrototypeGroupID: options.PrototypeGroupID,
		VariantID:        options.VariantID,
	}
	selected := make([]int64, 0, len(options.Variants))
	for _, variant := range options.Variants {
		if variant.VariantID > 0 {
			selected = append(selected, variant.VariantID)
		}
	}
	identity.SelectedVariantIDs = normalizedSDSBaselineVariantIDs(selected)
	return identity
}

func normalizedSDSBaselineVariantIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}

func ResolveSDSBaselineCacheScope(ctx context.Context, tenantID, baselineKey string) (string, string, string, error) {
	logicalKey := strings.TrimSpace(baselineKey)
	if logicalKey == "" {
		return "", "", "", nil
	}
	requestedTenant := strings.TrimSpace(tenantID)
	contextTenant, hasContextTenant := tenantctx.TenantScopeFromContext(ctx)
	if requestedTenant != "" {
		requestedTenant = tenantctx.NormalizeTenantID(requestedTenant)
	}
	if hasContextTenant && requestedTenant != "" && !tenantctx.MatchesTenant(requestedTenant, contextTenant) {
		return "", "", "", fmt.Errorf("%w: tenant argument %q context tenant %q", ErrSDSBaselineTenantMismatch, requestedTenant, contextTenant)
	}
	resolvedTenantID := requestedTenant
	if resolvedTenantID == "" {
		if hasContextTenant {
			resolvedTenantID = contextTenant
		} else {
			resolvedTenantID = tenantctx.DefaultTenantID
		}
	}
	return resolvedTenantID, logicalKey, storedSDSBaselineKey(resolvedTenantID, logicalKey), nil
}

func LogicalSDSBaselineKey(tenantID, storedKey string) string {
	resolvedTenantID := tenantctx.NormalizeTenantID(tenantID)
	if resolvedTenantID == tenantctx.DefaultTenantID {
		return strings.TrimSpace(storedKey)
	}
	prefix := resolvedTenantID + ":"
	return strings.TrimPrefix(strings.TrimSpace(storedKey), prefix)
}

func storedSDSBaselineKey(tenantID, baselineKey string) string {
	if strings.TrimSpace(baselineKey) == "" {
		return ""
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || tenantID == tenantctx.DefaultTenantID {
		return baselineKey
	}
	return tenantID + ":" + baselineKey
}
