package shein

import (
	"context"
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
