package listingkit

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) rememberSheinSubmittedPricing(task *Task, action string) {
	cacheStore := resolveSheinResolutionCacheStore(s)
	if s == nil || cacheStore == nil || task == nil || task.Result == nil || task.Result.Shein == nil || strings.TrimSpace(action) != "publish" {
		return
	}
	task.Result.Shein = sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	req := buildSheinPublishRequest(task.Request)
	review := normalizePublishedSheinPricingReview(task.Result.Shein)
	if review == nil {
		review = buildSheinDraftBackedPricingReview(task.Result.Shein, s.currentSheinPricingRule(), nil)
		review = normalizePublishedSheinPricingReview(&sheinpub.Package{
			DraftPayload: task.Result.Shein.DraftPayload,
			Pricing:      review,
		})
	}
	if req == nil || task.Result.Shein == nil || review == nil || !review.Ready {
		return
	}
	key := sheinPricingCacheKey(req, task.Result.Shein, s.currentSheinPricingRule())
	if key == "" {
		return
	}
	now := time.Now()
	attachPricingCacheInfo(review, "manual_cache", key, true, sheinpub.ResolutionCacheHitSourcePublishRemembered, "stored", 0, &now)
	task.Result.Shein.Pricing = review
	_ = cacheStore.SaveResolutionCache(context.Background(), &sheinpub.SheinResolutionCacheEntry{
		StoreID:        sheinPricingStoreID(req),
		CacheKind:      sheinpub.ResolutionCacheKindPricing,
		CacheKey:       key,
		ShortKey:       sheinPricingShortKey(key),
		Source:         "manual_cache",
		Manual:         true,
		SourceIdentity: sheinPricingSourceIdentity(task.Result.Shein),
		ResolutionJSON: mustMarshalSheinPricingReview(review),
		UpdatedAt:      now,
		CreatedAt:      now,
	})
	logPricingCacheEvent("store", req, task.Result.Shein, review.Cache, logrus.Fields{
		"cache_kind":         sheinpub.ResolutionCacheKindPricing,
		"product_identities": strings.Join(sheinPricingProductIdentity(task.Result.Shein), ","),
		"sku_facts":          strings.Join(sortedSheinPricingSKUFacts(task.Result.Shein, s.currentSheinPricingRule()), ","),
	})
}

func (s *service) loadSheinPricingCache(req *GenerateRequest, pkg *sheinpub.Package) *sheinpub.PricingReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	buildReq := buildSheinPublishRequest(req)
	if s == nil {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "service_nil"})
		return nil
	}
	if pkg == nil {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "package_nil"})
		return nil
	}
	cacheStore := resolveSheinResolutionCacheStore(s)
	if cacheStore == nil {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "no_resolution_cache_store"})
		return nil
	}
	key := sheinPricingCacheKey(buildReq, pkg, s.currentSheinPricingRule())
	if key == "" {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "empty_cache_key"})
		return nil
	}
	entry, err := cacheStore.GetResolutionCache(context.Background(), sheinpub.ResolutionCacheKindPricing, sheinPricingStoreID(buildReq), key)
	if err != nil {
		logPricingCacheEvent("error", buildReq, pkg, &sheinpub.ResolutionCacheInfo{
			CacheKey:  key,
			ShortKey:  sheinPricingShortKey(key),
			Clearable: key != "",
		}, logrus.Fields{
			"reason": "store_error",
			"error":  err.Error(),
		})
		return nil
	}
	if entry == nil {
		logPricingCacheEvent("miss", buildReq, pkg, &sheinpub.ResolutionCacheInfo{
			CacheKey:  key,
			ShortKey:  sheinPricingShortKey(key),
			Clearable: key != "",
		}, logrus.Fields{"reason": "no_entry"})
		return nil
	}
	review := decodeSheinPricingCacheEntry(entry)
	review = reconcileSheinPricingCacheReview(pkg, review)
	if !sheinPricingReviewApplicable(pkg, review) {
		logPricingCacheEvent("miss", buildReq, pkg, &sheinpub.ResolutionCacheInfo{
			Source:    cacheEntrySourceLabel(entry),
			HitSource: pricingCacheHitSource(entry),
			CacheKey:  entry.CacheKey,
			ShortKey:  sheinPricingShortKey(entry.CacheKey),
			HitCount:  entry.HitCount,
			Manual:    entry.Manual,
			Clearable: entry.CacheKey != "",
		}, logrus.Fields{"reason": "not_applicable"})
		return nil
	}
	attachPricingCacheInfo(review, cacheEntrySourceLabel(entry), entry.CacheKey, entry.Manual, pricingCacheHitSource(entry), "hit", entry.HitCount, &entry.UpdatedAt)
	logPricingCacheEvent("hit", buildReq, pkg, review.Cache, logrus.Fields{
		"cache_kind": sheinpub.ResolutionCacheKindPricing,
		"hit_count":  entry.HitCount,
	})
	return review
}

func (s *service) clearSheinPricingCache(req *sheinpub.BuildRequest, pkg *sheinpub.Package) error {
	cacheStore := resolveSheinResolutionCacheStore(s)
	if s == nil || cacheStore == nil {
		return nil
	}
	key := sheinPricingCacheKey(req, pkg, s.currentSheinPricingRule())
	if key == "" {
		return nil
	}
	if pkg != nil && pkg.Pricing != nil {
		pkg.Pricing.Cache = nil
	}
	return cacheStore.DeleteResolutionCache(context.Background(), sheinpub.ResolutionCacheKindPricing, sheinPricingStoreID(req), key)
}
