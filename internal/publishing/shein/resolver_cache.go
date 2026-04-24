package shein

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type cachedCategoryResolver struct {
	inner CategoryResolver
	cache sync.Map
	store ResolutionCacheStore
}

type cachedAttributeResolver struct {
	inner AttributeResolver
	cache sync.Map
	store ResolutionCacheStore
}

type cachedSaleAttributeResolver struct {
	inner SaleAttributeResolver
	cache sync.Map
	store ResolutionCacheStore
}

type CategoryResolutionCache interface {
	RememberCategoryResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *CategoryResolution)
	ClearCategoryResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error
}

type AttributeResolutionCache interface {
	RememberAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *AttributeResolution)
	ClearAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error
}

type SaleAttributeResolutionCache interface {
	RememberSaleAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *SaleAttributeResolution)
	ClearSaleAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error
}

func NewCachedCategoryResolver(inner CategoryResolver, stores ...ResolutionCacheStore) CategoryResolver {
	if inner == nil {
		return nil
	}
	return &cachedCategoryResolver{inner: inner, store: firstResolutionCacheStore(stores)}
}

func NewCachedAttributeResolver(inner AttributeResolver, stores ...ResolutionCacheStore) AttributeResolver {
	if inner == nil {
		return nil
	}
	return &cachedAttributeResolver{inner: inner, store: firstResolutionCacheStore(stores)}
}

func NewCachedSaleAttributeResolver(inner SaleAttributeResolver, stores ...ResolutionCacheStore) SaleAttributeResolver {
	if inner == nil {
		return nil
	}
	return &cachedSaleAttributeResolver{inner: inner, store: firstResolutionCacheStore(stores)}
}

func (r *cachedCategoryResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategoryResolution {
	if r == nil || r.inner == nil {
		return nil
	}
	key := categoryResolverCacheKey(req, canonical, pkg)
	if key != "" {
		if cached, ok := r.cache.Load(key); ok {
			if resolution, ok := cached.(*CategoryResolution); ok {
				return cloneCategoryResolutionWithCacheNote(resolution)
			}
		}
		if entry := r.loadPersistentCache(ResolutionCacheKindCategory, req, key); entry != nil {
			if resolution := decodeCategoryCacheEntry(entry); resolution != nil {
				r.cache.Store(key, cloneCategoryResolution(resolution))
				return resolution
			}
		}
	}
	resolution := r.inner.Resolve(req, canonical, pkg)
	if key != "" && shouldCacheCategoryResolution(resolution) {
		attachResolutionCacheInfoToCategory(resolution, normalizedResolutionSource(resolution.Source, "live_resolver"), key, false)
		r.cache.Store(key, cloneCategoryResolution(resolution))
		r.savePersistentCache(ResolutionCacheKindCategory, req, canonical, pkg, key, resolution, false)
	}
	return resolution
}

func (r *cachedCategoryResolver) RememberCategoryResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *CategoryResolution) {
	if r == nil || resolution == nil {
		return
	}
	key := categoryResolverCacheKey(req, canonical, pkg)
	if key == "" || !shouldCacheCategoryResolution(resolution) {
		return
	}
	attachResolutionCacheInfoToCategory(resolution, "manual_cache", key, true)
	r.cache.Store(key, cloneCategoryResolution(resolution))
	r.savePersistentCache(ResolutionCacheKindCategory, req, canonical, pkg, key, resolution, true)
}

func (r *cachedCategoryResolver) ClearCategoryResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error {
	if r == nil {
		return nil
	}
	key := categoryResolverCacheKey(req, canonical, pkg)
	return r.clearCache(ResolutionCacheKindCategory, req, key)
}

func (r *cachedCategoryResolver) SuggestAlternative(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategorySuggestion {
	if r == nil || r.inner == nil {
		return nil
	}
	recommender, ok := r.inner.(categoryRecommender)
	if !ok {
		return nil
	}
	return recommender.SuggestAlternative(req, canonical, pkg)
}

func (r *cachedAttributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *AttributeResolution {
	if r == nil || r.inner == nil {
		return nil
	}
	key := attributeResolverCacheKey(req, pkg)
	if key != "" {
		if cached, ok := r.cache.Load(key); ok {
			if resolution, ok := cached.(*AttributeResolution); ok {
				return cloneAttributeResolutionWithCacheNote(resolution)
			}
		}
		if entry := r.loadPersistentCache(ResolutionCacheKindAttribute, req, key); entry != nil {
			if resolution := decodeAttributeCacheEntry(entry); resolution != nil {
				r.cache.Store(key, cloneAttributeResolution(resolution))
				return resolution
			}
		}
	}
	resolution := r.inner.Resolve(req, canonical, pkg)
	if key != "" && shouldCacheAttributeResolution(resolution) {
		attachResolutionCacheInfoToAttribute(resolution, normalizedResolutionSource(resolution.Source, "live_resolver"), key, false)
		r.cache.Store(key, cloneAttributeResolution(resolution))
		r.savePersistentCache(ResolutionCacheKindAttribute, req, canonical, pkg, key, resolution, false)
	}
	return resolution
}

func (r *cachedAttributeResolver) RememberAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *AttributeResolution) {
	if r == nil || resolution == nil {
		return
	}
	key := attributeResolverCacheKey(req, pkg)
	if key == "" || !shouldCacheAttributeResolution(resolution) {
		return
	}
	attachResolutionCacheInfoToAttribute(resolution, "manual_cache", key, true)
	r.cache.Store(key, cloneAttributeResolution(resolution))
	r.savePersistentCache(ResolutionCacheKindAttribute, req, canonical, pkg, key, resolution, true)
}

func (r *cachedAttributeResolver) ClearAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error {
	if r == nil {
		return nil
	}
	key := attributeResolverCacheKey(req, pkg)
	return r.clearCache(ResolutionCacheKindAttribute, req, key)
}

func (r *cachedSaleAttributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *SaleAttributeResolution {
	if r == nil || r.inner == nil {
		return nil
	}
	key := saleAttributeResolverCacheKey(req, canonical, pkg)
	if key != "" {
		if cached, ok := r.cache.Load(key); ok {
			if resolution, ok := cached.(*SaleAttributeResolution); ok {
				return cloneSaleAttributeResolutionWithCacheNote(resolution)
			}
		}
		if entry := r.loadPersistentCache(ResolutionCacheKindSaleAttribute, req, key); entry != nil {
			if resolution := decodeSaleAttributeCacheEntry(entry); resolution != nil {
				r.cache.Store(key, cloneSaleAttributeResolution(resolution))
				return resolution
			}
		}
	}
	resolution := r.inner.Resolve(req, canonical, pkg)
	if key != "" && shouldCacheSaleAttributeResolution(resolution) {
		attachResolutionCacheInfoToSaleAttribute(resolution, normalizedResolutionSource(resolution.Source, "live_resolver"), key, false)
		r.cache.Store(key, cloneSaleAttributeResolution(resolution))
		r.savePersistentCache(ResolutionCacheKindSaleAttribute, req, canonical, pkg, key, resolution, false)
	}
	return resolution
}

func (r *cachedSaleAttributeResolver) RememberSaleAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, resolution *SaleAttributeResolution) {
	if r == nil || resolution == nil {
		return
	}
	key := saleAttributeResolverCacheKey(req, canonical, pkg)
	if key == "" || !shouldCacheSaleAttributeResolution(resolution) {
		return
	}
	attachResolutionCacheInfoToSaleAttribute(resolution, "manual_cache", key, true)
	r.cache.Store(key, cloneSaleAttributeResolution(resolution))
	r.savePersistentCache(ResolutionCacheKindSaleAttribute, req, canonical, pkg, key, resolution, true)
}

func (r *cachedSaleAttributeResolver) ClearSaleAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error {
	if r == nil {
		return nil
	}
	key := saleAttributeResolverCacheKey(req, canonical, pkg)
	return r.clearCache(ResolutionCacheKindSaleAttribute, req, key)
}

func (r *cachedCategoryResolver) loadPersistentCache(kind string, req *BuildRequest, key string) *SheinResolutionCacheEntry {
	if r == nil || r.store == nil {
		return nil
	}
	entry, _ := r.store.GetResolutionCache(context.Background(), kind, sheinStoreID(req), key)
	return entry
}

func (r *cachedAttributeResolver) loadPersistentCache(kind string, req *BuildRequest, key string) *SheinResolutionCacheEntry {
	if r == nil || r.store == nil {
		return nil
	}
	entry, _ := r.store.GetResolutionCache(context.Background(), kind, sheinStoreID(req), key)
	return entry
}

func (r *cachedSaleAttributeResolver) loadPersistentCache(kind string, req *BuildRequest, key string) *SheinResolutionCacheEntry {
	if r == nil || r.store == nil {
		return nil
	}
	entry, _ := r.store.GetResolutionCache(context.Background(), kind, sheinStoreID(req), key)
	return entry
}

func (r *cachedCategoryResolver) savePersistentCache(kind string, req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, key string, resolution any, manual bool) {
	if r == nil || r.store == nil {
		return
	}
	_ = r.store.SaveResolutionCache(context.Background(), buildResolutionCacheEntry(kind, req, canonical, pkg, key, resolution, manual))
}

func (r *cachedCategoryResolver) clearCache(kind string, req *BuildRequest, key string) error {
	if key == "" {
		return nil
	}
	r.cache.Delete(key)
	if r.store == nil {
		return nil
	}
	return r.store.DeleteResolutionCache(context.Background(), kind, sheinStoreID(req), key)
}

func (r *cachedAttributeResolver) savePersistentCache(kind string, req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, key string, resolution any, manual bool) {
	if r == nil || r.store == nil {
		return
	}
	_ = r.store.SaveResolutionCache(context.Background(), buildResolutionCacheEntry(kind, req, canonical, pkg, key, resolution, manual))
}

func (r *cachedAttributeResolver) clearCache(kind string, req *BuildRequest, key string) error {
	if key == "" {
		return nil
	}
	r.cache.Delete(key)
	if r.store == nil {
		return nil
	}
	return r.store.DeleteResolutionCache(context.Background(), kind, sheinStoreID(req), key)
}

func (r *cachedSaleAttributeResolver) savePersistentCache(kind string, req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, key string, resolution any, manual bool) {
	if r == nil || r.store == nil {
		return
	}
	_ = r.store.SaveResolutionCache(context.Background(), buildResolutionCacheEntry(kind, req, canonical, pkg, key, resolution, manual))
}

func (r *cachedSaleAttributeResolver) clearCache(kind string, req *BuildRequest, key string) error {
	if key == "" {
		return nil
	}
	r.cache.Delete(key)
	if r.store == nil {
		return nil
	}
	return r.store.DeleteResolutionCache(context.Background(), kind, sheinStoreID(req), key)
}

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

func buildResolutionCacheEntry(kind string, req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, key string, resolution any, manual bool) *SheinResolutionCacheEntry {
	data, err := json.Marshal(resolution)
	if err != nil {
		return nil
	}
	now := time.Now()
	source := "live_resolver"
	if manual {
		source = "manual_cache"
	} else {
		source = resolutionCacheSourceFromValue(resolution)
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

func buildResolutionCacheSourceIdentity(kind string, canonical *productenrich.CanonicalProduct, pkg *Package) string {
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
		ShortKey:  shortResolutionCacheKey(key),
		UpdatedAt: &now,
		Manual:    manual,
		Clearable: key != "",
	}
}

func shortResolutionCacheKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

func categoryResolverCacheKey(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	payload := map[string]any{
		"version":          2,
		"store_id":         sheinStoreID(req),
		"category_path":    normalizedSourceCategoryPath(canonical, pkg),
		"product_identity": stableProductIdentity(canonical, pkg),
	}
	return hashCachePayload(payload)
}

func attributeResolverCacheKey(req *BuildRequest, pkg *Package) string {
	if pkg == nil || categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":            1,
		"store_id":           sheinStoreID(req),
		"category_id":        categoryID(pkg),
		"category_id_list":   append([]int(nil), pkg.CategoryIDList...),
		"product_attributes": normalizedAttributeInputs(buildAttributeInputs(pkg)),
	}
	return hashCachePayload(payload)
}

func saleAttributeResolverCacheKey(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	if categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":           1,
		"store_id":          sheinStoreID(req),
		"category_id":       categoryID(pkg),
		"category_id_list":  append([]int(nil), pkg.CategoryIDList...),
		"source_dimensions": normalizedSourceDimensions(canonical),
	}
	return hashCachePayload(payload)
}

func shouldCacheCategoryResolution(resolution *CategoryResolution) bool {
	return resolution != nil && resolution.CategoryID > 0 && strings.TrimSpace(resolution.Status) != "unresolved"
}

func shouldCacheAttributeResolution(resolution *AttributeResolution) bool {
	return resolution != nil && resolution.TemplateCount > 0 && resolution.ResolvedCount > 0
}

func shouldCacheSaleAttributeResolution(resolution *SaleAttributeResolution) bool {
	if resolution == nil {
		return false
	}
	return resolution.PrimaryAttributeID > 0 || resolution.SecondaryAttributeID > 0 || len(resolution.SKCAttributes) > 0 || len(resolution.SKUAttributes) > 0
}

func normalizedSourceDimensions(canonical *productenrich.CanonicalProduct) []string {
	dimensions := buildSourceVariantDimensions(canonical, common.BuildVariants(canonical))
	if len(dimensions) == 0 {
		return nil
	}
	result := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		name := normalizeText(dimension.Name)
		if name == "" {
			continue
		}
		values := make([]string, 0, len(dimension.Values))
		for _, value := range dimension.Values {
			value = normalizeText(value)
			if value != "" {
				values = append(values, value)
			}
		}
		sort.Strings(values)
		result = append(result, name+"="+strings.Join(values, "|"))
	}
	sort.Strings(result)
	return result
}

func normalizedSourceCategoryPath(canonical *productenrich.CanonicalProduct, pkg *Package) []string {
	var path []string
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		path = canonical.CategoryPath
	} else {
		path = resolveCategoryPath(canonical, pkg)
	}
	result := make([]string, 0, len(path))
	for _, item := range path {
		item = normalizeText(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func stableProductIdentity(canonical *productenrich.CanonicalProduct, pkg *Package) []string {
	values := make([]string, 0, 6)
	if canonical != nil {
		values = append(values, canonical.Title)
	}
	if pkg != nil {
		values = append(values, pkg.SpuName, pkg.ProductNameEn, lookupAttributeValueInList(pkg.ProductAttributes, "sku"), lookupAttributeValueInList(pkg.ProductAttributes, "parent sku"))
	}
	for idx := range values {
		values[idx] = normalizeText(values[idx])
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizedAttributeInputs(inputs []common.Attribute) []string {
	if len(inputs) == 0 {
		return nil
	}
	result := make([]string, 0, len(inputs))
	for _, item := range inputs {
		name := normalizeText(item.Name)
		value := normalizeText(item.Value)
		if name == "" || value == "" {
			continue
		}
		result = append(result, name+"="+value)
	}
	sort.Strings(result)
	return result
}

func lookupAttributeValueInList(inputs []common.Attribute, name string) string {
	name = normalizeText(name)
	for _, item := range inputs {
		if normalizeText(item.Name) == name {
			return item.Value
		}
	}
	return ""
}

func sheinStoreID(req *BuildRequest) string {
	if req == nil || req.SheinStoreID == 0 {
		return ""
	}
	return strconv.FormatInt(req.SheinStoreID, 10)
}

func hashCachePayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneCategoryResolutionWithCacheNote(resolution *CategoryResolution) *CategoryResolution {
	clone := cloneCategoryResolution(resolution)
	if clone != nil {
		if clone.Cache != nil && clone.Cache.Source != "manual_cache" {
			clone.Cache.Source = "memory_cache"
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 类目缓存命中: 已复用同一底版商品的人工/历史类目解析结果")
	}
	return clone
}

func cloneAttributeResolutionWithCacheNote(resolution *AttributeResolution) *AttributeResolution {
	clone := cloneAttributeResolution(resolution)
	if clone != nil {
		if clone.Cache != nil && clone.Cache.Source != "manual_cache" {
			clone.Cache.Source = "memory_cache"
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 普通属性缓存命中: 已复用同一底版商品的人工/历史属性解析结果")
	}
	return clone
}

func cloneSaleAttributeResolutionWithCacheNote(resolution *SaleAttributeResolution) *SaleAttributeResolution {
	clone := cloneSaleAttributeResolution(resolution)
	if clone != nil {
		if clone.Cache != nil && clone.Cache.Source != "manual_cache" {
			clone.Cache.Source = "memory_cache"
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 销售属性缓存命中: 已复用同一底版商品的人工/历史销售属性解析结果")
	}
	return clone
}

func cloneCategoryResolution(resolution *CategoryResolution) *CategoryResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.MatchedPath = append([]string(nil), resolution.MatchedPath...)
	clone.CategoryIDList = append([]int(nil), resolution.CategoryIDList...)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	clone.SuggestedCategory = cloneCategorySuggestion(resolution.SuggestedCategory)
	if resolution.SemanticValidation != nil {
		semantic := *resolution.SemanticValidation
		semantic.ComparedPath = append([]string(nil), resolution.SemanticValidation.ComparedPath...)
		clone.SemanticValidation = &semantic
	}
	return &clone
}

func cloneCategorySuggestion(suggestion *CategorySuggestion) *CategorySuggestion {
	if suggestion == nil {
		return nil
	}
	clone := *suggestion
	clone.MatchedPath = append([]string(nil), suggestion.MatchedPath...)
	clone.CategoryIDList = append([]int(nil), suggestion.CategoryIDList...)
	return &clone
}

func cloneAttributeResolution(resolution *AttributeResolution) *AttributeResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	clone.PendingAttributes = append([]common.Attribute(nil), resolution.PendingAttributes...)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	return &clone
}

func cloneSaleAttributeResolution(resolution *SaleAttributeResolution) *SaleAttributeResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.SourceDimensions = cloneSourceVariantDimensions(resolution.SourceDimensions)
	clone.SKCAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKCAttributes...)
	clone.SKUAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKUAttributes...)
	clone.Candidates = cloneSaleAttributeCandidateInfos(resolution.Candidates)
	clone.SelectionSummary = append([]string(nil), resolution.SelectionSummary...)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.CustomAttributeRelation = append([]sheinattribute.CustomAttributeRelation(nil), resolution.CustomAttributeRelation...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	clone.skcAssignments = cloneResolvedSaleAttributeMap(resolution.skcAssignments)
	clone.skuAssignments = cloneResolvedSaleAttributeSliceMap(resolution.skuAssignments)
	clone.skcValueAssignments = cloneResolvedSaleAttributeMap(resolution.skcValueAssignments)
	clone.skuValueAssignments = cloneResolvedSaleAttributeMap(resolution.skuValueAssignments)
	return &clone
}

func cloneResolutionCacheInfo(info *ResolutionCacheInfo) *ResolutionCacheInfo {
	if info == nil {
		return nil
	}
	clone := *info
	if info.UpdatedAt != nil {
		updatedAt := *info.UpdatedAt
		clone.UpdatedAt = &updatedAt
	}
	return &clone
}

func cloneSourceVariantDimensions(items []SourceVariantDimension) []SourceVariantDimension {
	if len(items) == 0 {
		return nil
	}
	out := make([]SourceVariantDimension, 0, len(items))
	for _, item := range items {
		item.Values = append([]string(nil), item.Values...)
		out = append(out, item)
	}
	return out
}

func cloneSaleAttributeCandidateInfos(items []SaleAttributeCandidateInfo) []SaleAttributeCandidateInfo {
	if len(items) == 0 {
		return nil
	}
	out := make([]SaleAttributeCandidateInfo, 0, len(items))
	for _, item := range items {
		item.Reasons = append([]string(nil), item.Reasons...)
		out = append(out, item)
	}
	return out
}

func cloneResolvedSaleAttributeMap(input map[string]ResolvedSaleAttribute) map[string]ResolvedSaleAttribute {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]ResolvedSaleAttribute, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneResolvedSaleAttributeSliceMap(input map[string][]ResolvedSaleAttribute) map[string][]ResolvedSaleAttribute {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]ResolvedSaleAttribute, len(input))
	for key, value := range input {
		out[key] = append([]ResolvedSaleAttribute(nil), value...)
	}
	return out
}
