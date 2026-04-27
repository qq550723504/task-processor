package shein

import (
	"context"
	"strings"
	"sync"

	"task-processor/internal/productenrich"
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
	return r.clearCacheWithInfo(ResolutionCacheKindCategory, req, key, categoryResolutionCacheInfo(pkg))
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
	existingKey := ""
	if resolution.Cache != nil {
		existingKey = strings.TrimSpace(resolution.Cache.CacheKey)
	}
	key := attributeResolverCacheKey(req, pkg)
	if key == "" || !shouldCacheAttributeResolution(resolution) {
		return
	}
	attachResolutionCacheInfoToAttribute(resolution, "manual_cache", key, true)
	r.cache.Store(key, cloneAttributeResolution(resolution))
	r.savePersistentCache(ResolutionCacheKindAttribute, req, canonical, pkg, key, resolution, true)
	if existingKey != "" && existingKey != key {
		attachResolutionCacheInfoToAttribute(resolution, "manual_cache", existingKey, true)
		r.cache.Store(existingKey, cloneAttributeResolution(resolution))
		r.savePersistentCache(ResolutionCacheKindAttribute, req, canonical, pkg, existingKey, resolution, true)
		attachResolutionCacheInfoToAttribute(resolution, "manual_cache", key, true)
	}
}

func (r *cachedAttributeResolver) ClearAttributeResolution(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) error {
	if r == nil {
		return nil
	}
	key := attributeResolverCacheKey(req, pkg)
	return r.clearCacheWithInfo(ResolutionCacheKindAttribute, req, key, attributeResolutionCacheInfo(pkg))
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
	return r.clearCacheWithInfo(ResolutionCacheKindSaleAttribute, req, key, saleAttributeResolutionCacheInfo(pkg))
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

func (r *cachedCategoryResolver) clearCacheWithInfo(kind string, req *BuildRequest, key string, info *ResolutionCacheInfo) error {
	if r == nil {
		return nil
	}
	return clearResolutionCacheEntries(&r.cache, r.store, kind, sheinStoreID(req), key, info)
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

func (r *cachedAttributeResolver) clearCacheWithInfo(kind string, req *BuildRequest, key string, info *ResolutionCacheInfo) error {
	if r == nil {
		return nil
	}
	return clearResolutionCacheEntries(&r.cache, r.store, kind, sheinStoreID(req), key, info)
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

func (r *cachedSaleAttributeResolver) clearCacheWithInfo(kind string, req *BuildRequest, key string, info *ResolutionCacheInfo) error {
	if r == nil {
		return nil
	}
	return clearResolutionCacheEntries(&r.cache, r.store, kind, sheinStoreID(req), key, info)
}

func clearResolutionCacheEntries(cache *sync.Map, store ResolutionCacheStore, kind string, storeID string, computedKey string, info *ResolutionCacheInfo) error {
	keys := uniqueResolutionCacheKeys(computedKey, info)
	for _, key := range keys {
		cache.Delete(key)
	}
	shortKey := resolutionCacheShortKey(info)
	if shortKey != "" {
		cache.Range(func(key, _ any) bool {
			if text, ok := key.(string); ok && shortResolutionCacheKey(text) == shortKey {
				cache.Delete(text)
			}
			return true
		})
	}
	if store == nil {
		return nil
	}
	for _, key := range keys {
		if err := store.DeleteResolutionCache(context.Background(), kind, storeID, key); err != nil {
			return err
		}
	}
	if shortKey != "" {
		if deleter, ok := store.(ResolutionCacheShortKeyDeleter); ok {
			if err := deleter.DeleteResolutionCacheByShortKey(context.Background(), kind, storeID, shortKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func uniqueResolutionCacheKeys(computedKey string, info *ResolutionCacheInfo) []string {
	seen := make(map[string]struct{}, 2)
	keys := make([]string, 0, 2)
	for _, key := range []string{computedKey, resolutionCacheFullKey(info)} {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func resolutionCacheFullKey(info *ResolutionCacheInfo) string {
	if info == nil {
		return ""
	}
	return info.CacheKey
}

func resolutionCacheShortKey(info *ResolutionCacheInfo) string {
	if info == nil {
		return ""
	}
	return info.ShortKey
}

func categoryResolutionCacheInfo(pkg *Package) *ResolutionCacheInfo {
	if pkg == nil || pkg.CategoryResolution == nil {
		return nil
	}
	return pkg.CategoryResolution.Cache
}

func attributeResolutionCacheInfo(pkg *Package) *ResolutionCacheInfo {
	if pkg == nil || pkg.AttributeResolution == nil {
		return nil
	}
	return pkg.AttributeResolution.Cache
}

func saleAttributeResolutionCacheInfo(pkg *Package) *ResolutionCacheInfo {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return nil
	}
	return pkg.SaleAttributeResolution.Cache
}
