package shein

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

func SizeAttributeCacheKey(req *BuildRequest, pkg *Package) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SizeAttributeList) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":          1,
		"store_id":         SizeAttributeStoreID(req),
		"category_id":      pkg.CategoryID,
		"category_id_list": append([]int(nil), pkg.CategoryIDList...),
		"category_path":    normalizedPricingTextList(pkg.CategoryPath),
		"product_identity": StablePricingPackageIdentity(pkg),
		"size_aliases":     SortedSizeAttributeAliases(pkg),
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func SizeAttributeStoreID(req *BuildRequest) string {
	if req == nil || req.SheinStoreID == 0 {
		return ""
	}
	return strconv.FormatInt(req.SheinStoreID, 10)
}

func SizeAttributeShortKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

func SizeAttributeSourceIdentity(pkg *Package) string {
	payload := map[string]any{
		"category_path":    normalizedPricingTextList(pkg.CategoryPath),
		"product_identity": StablePricingPackageIdentity(pkg),
		"size_aliases":     SortedSizeAttributeAliases(pkg),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func SortedSizeAttributeFacts(pkg *Package) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SizeAttributeList) == 0 {
		return nil
	}
	result := make([]string, 0, len(pkg.DraftPayload.SizeAttributeList))
	for _, attr := range pkg.DraftPayload.SizeAttributeList {
		key := sizeAttributeCacheKey(attr)
		if key == "" {
			continue
		}
		result = append(result, key+"="+strings.TrimSpace(attr.AttributeExtraValue))
	}
	sort.Strings(result)
	return result
}

func SortedSizeAttributeAliases(pkg *Package) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SizeAttributeList) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, attr := range pkg.DraftPayload.SizeAttributeList {
		alias := sizeAttributeCacheAlias(attr)
		if alias != "" {
			seen[alias] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for alias := range seen {
		result = append(result, alias)
	}
	sort.Strings(result)
	return result
}

func NormalizePublishedSizeAttributeReview(pkg *Package) *SizeAttributeReview {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SizeAttributeList) == 0 {
		return nil
	}
	review := &SizeAttributeReview{
		Attributes: append([]sheinproduct.SizeAttribute(nil), pkg.DraftPayload.SizeAttributeList...),
		Ready:      true,
		UpdatedAt:  ptrTime(time.Now()),
	}
	review.ManualOverrides = buildSizeAttributeManualOverrides(review.Attributes)
	return review
}

func DecodeSizeAttributeCacheEntry(entry *SheinResolutionCacheEntry) *SizeAttributeReview {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var review SizeAttributeReview
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &review); err != nil {
		return nil
	}
	return CloneSizeAttributeReview(&review)
}

func CloneSizeAttributeReview(review *SizeAttributeReview) *SizeAttributeReview {
	if review == nil {
		return nil
	}
	cloned := *review
	if len(review.Attributes) > 0 {
		cloned.Attributes = append([]sheinproduct.SizeAttribute(nil), review.Attributes...)
	}
	if len(review.ManualOverrides) > 0 {
		cloned.ManualOverrides = cloneSizeAttributeOverrides(review.ManualOverrides)
	}
	if review.UpdatedAt != nil {
		updatedAt := *review.UpdatedAt
		cloned.UpdatedAt = &updatedAt
	}
	if review.Cache != nil {
		cloned.Cache = CloneResolutionCacheInfo(review.Cache)
	}
	return &cloned
}

func SizeAttributeReviewApplicable(pkg *Package, review *SizeAttributeReview) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || review == nil || !review.Ready || len(review.Attributes) == 0 {
		return false
	}
	current := SortedSizeAttributeAliases(pkg)
	if len(current) == 0 || len(current) != len(SortedSizeAttributeAliasesFromAttributes(review.Attributes)) {
		return false
	}
	reviewAliases := SortedSizeAttributeAliasesFromAttributes(review.Attributes)
	for idx := range current {
		if current[idx] != reviewAliases[idx] {
			return false
		}
	}
	return true
}

func SortedSizeAttributeAliasesFromAttributes(attrs []sheinproduct.SizeAttribute) []string {
	seen := map[string]struct{}{}
	for _, attr := range attrs {
		alias := sizeAttributeCacheAlias(attr)
		if alias != "" {
			seen[alias] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for alias := range seen {
		result = append(result, alias)
	}
	sort.Strings(result)
	return result
}

func ReconcileSizeAttributeCacheReview(pkg *Package, review *SizeAttributeReview) *SizeAttributeReview {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || review == nil {
		return review
	}
	current := pkg.DraftPayload.SizeAttributeList
	if len(current) == 0 {
		return review
	}

	manualByAlias := sizeAttributeManualOverridesByAlias(review.ManualOverrides)
	remappedManuals := make(map[string]string, len(current))
	occurrence := map[string]int{}
	attrs := make([]sheinproduct.SizeAttribute, 0, len(current))
	for _, attr := range current {
		alias := sizeAttributeCacheAlias(attr)
		if alias == "" {
			continue
		}
		candidate := attr
		if value, ok := manualByAlias[alias]; ok {
			candidate.AttributeExtraValue = value
			remappedManuals[sizeAttributeCacheKey(candidate)] = value
		} else if cached, ok := nextCachedSizeAttribute(review.Attributes, alias, occurrence); ok {
			candidate.AttributeExtraValue = cached.AttributeExtraValue
		}
		attrs = append(attrs, candidate)
	}
	review.Attributes = attrs
	if len(remappedManuals) > 0 {
		review.ManualOverrides = remappedManuals
	}
	return review
}

func ApplySizeAttributeReview(pkg *Package, review *SizeAttributeReview) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || review == nil {
		return
	}
	pkg.DraftPayload.SizeAttributeList = append([]sheinproduct.SizeAttribute(nil), review.Attributes...)
	if pkg.PreviewPayload != nil {
		pkg.PreviewPayload.SizeAttributeList = append([]sheinproduct.SizeAttribute(nil), review.Attributes...)
	}
	SetPreviewPayload(pkg, pkg.PreviewPayload)
}

func nextCachedSizeAttribute(attrs []sheinproduct.SizeAttribute, alias string, occurrence map[string]int) (sheinproduct.SizeAttribute, bool) {
	index := occurrence[alias]
	for _, attr := range attrs {
		if sizeAttributeCacheAlias(attr) != alias {
			continue
		}
		if index == 0 {
			occurrence[alias]++
			return attr, true
		}
		index--
	}
	return sheinproduct.SizeAttribute{}, false
}

func sizeAttributeManualOverridesByAlias(overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range overrides {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		alias := sizeAttributeCacheAliasFromKey(key)
		if alias == "" {
			continue
		}
		result[alias] = value
	}
	return result
}

func buildSizeAttributeManualOverrides(attrs []sheinproduct.SizeAttribute) map[string]string {
	result := map[string]string{}
	for _, attr := range attrs {
		key := sizeAttributeCacheKey(attr)
		value := strings.TrimSpace(attr.AttributeExtraValue)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneSizeAttributeOverrides(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func sizeAttributeCacheKey(attr sheinproduct.SizeAttribute) string {
	if attr.AttributeID <= 0 || attr.RelateSaleAttributeID <= 0 || attr.RelateSaleAttributeValueID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d|%d", attr.RelateSaleAttributeID, attr.RelateSaleAttributeValueID, attr.AttributeID)
}

func sizeAttributeCacheAlias(attr sheinproduct.SizeAttribute) string {
	if attr.AttributeID <= 0 || attr.RelateSaleAttributeID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d|%d", attr.RelateSaleAttributeID, attr.AttributeID)
}

func sizeAttributeCacheAliasFromKey(key string) string {
	left, attrID, ok := strings.Cut(strings.TrimSpace(key), "|")
	if !ok || strings.TrimSpace(attrID) == "" {
		return ""
	}
	saleID, _, ok := strings.Cut(left, ":")
	if !ok || strings.TrimSpace(saleID) == "" {
		return ""
	}
	return strings.TrimSpace(saleID) + "|" + strings.TrimSpace(attrID)
}
