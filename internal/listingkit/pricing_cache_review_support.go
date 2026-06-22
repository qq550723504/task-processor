package listingkit

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	sheinpub "task-processor/internal/publishing/shein"
)

func normalizePublishedSheinPricingReview(pkg *sheinpub.Package) *sheinpub.PricingReview {
	return sheinpub.NormalizePublishedPricingReview(pkg)
}

func sheinPricingReviewApplicable(pkg *sheinpub.Package, review *sheinpub.PricingReview) bool {
	return sheinpub.PricingReviewApplicable(pkg, review)
}

func mustMarshalSheinPricingReview(review *sheinpub.PricingReview) string {
	data, err := json.Marshal(review)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeSheinPricingCacheEntry(entry *sheinpub.SheinResolutionCacheEntry) *sheinpub.PricingReview {
	return sheinpub.DecodePricingCacheEntry(entry)
}

func reconcileSheinPricingCacheReview(pkg *sheinpub.Package, review *sheinpub.PricingReview) *sheinpub.PricingReview {
	return sheinpub.ReconcilePricingCacheReview(pkg, review)
}

func cloneSheinPricingReview(review *sheinpub.PricingReview) *sheinpub.PricingReview {
	return sheinpub.ClonePricingReview(review)
}

func attachPricingCacheInfo(
	review *sheinpub.PricingReview,
	source string,
	key string,
	manual bool,
	hitSource string,
	status string,
	hitCount int,
	updatedAt *time.Time,
) {
	if review == nil {
		return
	}
	info := &sheinpub.ResolutionCacheInfo{
		Status:    pricingCacheStatusForSource(source, status),
		Source:    source,
		HitSource: hitSource,
		CacheKey:  key,
		ShortKey:  sheinpub.PricingShortKey(key),
		HitCount:  hitCount,
		Manual:    manual,
		Clearable: key != "",
	}
	if updatedAt != nil {
		copyUpdatedAt := *updatedAt
		info.UpdatedAt = &copyUpdatedAt
	}
	review.Cache = info
}

func pricingCacheStatusForSource(source string, status string) string {
	if strings.TrimSpace(status) != "" {
		return status
	}
	switch source {
	case "manual_cache", "history_cache", "memory_cache":
		return "hit"
	default:
		return "stored"
	}
}

func cacheEntrySourceLabel(entry *sheinpub.SheinResolutionCacheEntry) string {
	if entry != nil && entry.Manual {
		return "manual_cache"
	}
	return "history_cache"
}

func pricingCacheHitSource(entry *sheinpub.SheinResolutionCacheEntry) string {
	if entry != nil && entry.Manual {
		return sheinpub.ResolutionCacheHitSourcePersistentManualCache
	}
	return sheinpub.ResolutionCacheHitSourcePersistentHistoryCache
}

func logPricingCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {
	// TODO(debug): Remove this verbose cache-key diagnostics log after repeated publish cache verification is complete.
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/pricing_cache",
		"event":     event,
		"store_id":  sheinpub.PricingStoreID(req),
		"category_id": func() int {
			if pkg == nil {
				return 0
			}
			return pkg.CategoryID
		}(),
		"product_identities": func() string {
			if pkg == nil {
				return ""
			}
			return strings.Join(sheinpub.PricingProductIdentity(pkg), ",")
		}(),
		"sku_facts": func() string {
			if pkg == nil {
				return ""
			}
			return strings.Join(sheinpub.SortedPricingSKUFacts(pkg, sheinpub.PricingRule{}), ",")
		}(),
	})
	for key, value := range fields {
		log = log.WithField(key, value)
	}
	if info != nil {
		log = log.WithFields(logrus.Fields{
			"cache_source": info.Source,
			"cache_key":    info.CacheKey,
			"short_key":    info.ShortKey,
		})
	}
	log.Info("processed SHEIN pricing cache")
}
