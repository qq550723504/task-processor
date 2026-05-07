package shein

import (
	"encoding/json"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
)

func firstResolutionCacheStore(stores []ResolutionCacheStore) ResolutionCacheStore {
	for _, store := range stores {
		if store != nil {
			return store
		}
	}
	return nil
}

func decodeCategoryCacheEntry(entry *SheinResolutionCacheEntry) *CategoryResolution {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var resolution CategoryResolution
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &resolution); err != nil {
		return nil
	}
	attachResolutionCacheInfoToCategory(&resolution, cacheEntrySource(entry), entry.CacheKey, entry.Manual)
	if resolution.Cache != nil {
		resolution.Cache.HitCount = entry.HitCount
		resolution.Cache.UpdatedAt = &entry.UpdatedAt
	}
	return &resolution
}

func decodeAttributeCacheEntry(entry *SheinResolutionCacheEntry) *AttributeResolution {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var resolution AttributeResolution
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &resolution); err != nil {
		return nil
	}
	attachResolutionCacheInfoToAttribute(&resolution, cacheEntrySource(entry), entry.CacheKey, entry.Manual)
	if resolution.Cache != nil {
		resolution.Cache.HitCount = entry.HitCount
		resolution.Cache.UpdatedAt = &entry.UpdatedAt
	}
	return &resolution
}

func decodeSaleAttributeCacheEntry(entry *SheinResolutionCacheEntry) *SaleAttributeResolution {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var resolution SaleAttributeResolution
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &resolution); err != nil {
		return nil
	}
	attachResolutionCacheInfoToSaleAttribute(&resolution, cacheEntrySource(entry), entry.CacheKey, entry.Manual)
	if resolution.Cache != nil {
		resolution.Cache.HitCount = entry.HitCount
		resolution.Cache.UpdatedAt = &entry.UpdatedAt
	}
	return &resolution
}

func cacheEntrySource(entry *SheinResolutionCacheEntry) string {
	if entry != nil && entry.Manual {
		return "manual_cache"
	}
	return "history_cache"
}

func buildResolutionCacheEntry(kind string, req *BuildRequest, canonical *canonical.Product, pkg *Package, key string, resolution any, manual bool) *SheinResolutionCacheEntry {
	data, err := json.Marshal(resolution)
	if err != nil {
		return nil
	}
	now := time.Now()
	source := resolutionCacheSourceFromValue(resolution)
	if manual {
		source = "manual_cache"
	}
	return &SheinResolutionCacheEntry{
		StoreID:        sheinStoreID(req),
		CacheKind:      kind,
		CacheKey:       key,
		ShortKey:       shortResolutionCacheKey(key),
		Source:         source,
		Manual:         manual,
		SourceIdentity: buildResolutionCacheSourceIdentity(kind, canonical, pkg),
		ResolutionJSON: string(data),
		UpdatedAt:      now,
		CreatedAt:      now,
	}
}

func resolutionCacheSourceFromValue(resolution any) string {
	switch value := resolution.(type) {
	case *CategoryResolution:
		return normalizedResolutionSource(value.Source, "live_resolver")
	case *AttributeResolution:
		return normalizedResolutionSource(value.Source, "live_resolver")
	case *SaleAttributeResolution:
		return normalizedResolutionSource(value.Source, "live_resolver")
	default:
		return "live_resolver"
	}
}

func normalizedResolutionSource(source string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "manual_cache", "history_cache", "memory_cache", "live_resolver", "static_fallback", "llm":
		return strings.ToLower(strings.TrimSpace(source))
	case "llm_category_resolver", "llm_attribute_resolver", "llm_sale_attribute_resolver":
		return "llm"
	case "static", "static_matcher":
		return "static_fallback"
	default:
		if fallback == "" {
			return "live_resolver"
		}
		return fallback
	}
}

func buildResolutionCacheSourceIdentity(kind string, canonical *canonical.Product, pkg *Package) string {
	payload := map[string]any{
		"kind":              kind,
		"category_path":     normalizedSourceCategoryPath(canonical, pkg),
		"product_identity":  stableProductIdentity(canonical, pkg),
		"category_id":       categoryID(pkg),
		"category_id_list":  nil,
		"source_dimensions": normalizedSourceDimensions(canonical),
	}
	if pkg != nil {
		payload["category_id_list"] = append([]int(nil), pkg.CategoryIDList...)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func attachResolutionCacheInfoToCategory(resolution *CategoryResolution, source string, key string, manual bool) {
	if resolution == nil {
		return
	}
	resolution.Cache = buildResolutionCacheInfo(source, key, manual)
}

func attachResolutionCacheInfoToAttribute(resolution *AttributeResolution, source string, key string, manual bool) {
	if resolution == nil {
		return
	}
	resolution.Cache = buildResolutionCacheInfo(source, key, manual)
}

func attachResolutionCacheInfoToSaleAttribute(resolution *SaleAttributeResolution, source string, key string, manual bool) {
	if resolution == nil {
		return
	}
	resolution.Cache = buildResolutionCacheInfo(source, key, manual)
}

func buildResolutionCacheInfo(source string, key string, manual bool) *ResolutionCacheInfo {
	now := time.Now()
	status := "stored"
	switch source {
	case "memory_cache", "history_cache", "manual_cache":
		status = "hit"
	case "live_resolver", "static_fallback", "llm":
		status = "stored"
	}
	return &ResolutionCacheInfo{
		Status:    status,
		Source:    source,
		CacheKey:  key,
		ShortKey:  shortResolutionCacheKey(key),
		UpdatedAt: &now,
		Manual:    manual,
		Clearable: key != "",
	}
}
