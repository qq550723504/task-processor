package listingkit

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	sheinpub "task-processor/internal/publishing/shein"
)

func normalizePublishedSheinPricingReview(pkg *sheinpub.Package) *sheinpub.PricingReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	review := cloneSheinPricingReview(pkg.Pricing)
	if review == nil || !review.Ready || len(review.SKUPrices) == 0 {
		return nil
	}
	for idx := range review.SKUPrices {
		price := &review.SKUPrices[idx]
		if strings.TrimSpace(price.SupplierSKU) == "" || price.FinalPrice <= 0 {
			return nil
		}
		price.SupplierSKU = strings.TrimSpace(price.SupplierSKU)
		price.SupplierCode = strings.TrimSpace(price.SupplierCode)
	}
	return review
}

func sheinPricingReviewApplicable(pkg *sheinpub.Package, review *sheinpub.PricingReview) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || review == nil || !review.Ready || len(review.SKUPrices) == 0 {
		return false
	}
	current := sheinPricingSKUFacts(pkg, sheinpub.PricingRule{})
	if len(current) == 0 || len(current) != len(review.SKUPrices) {
		return false
	}
	for _, item := range review.SKUPrices {
		key := sheinPricingReviewSKUKey(item)
		fact, ok := current[key]
		if !ok || fact.CostPrice != formatMoney(item.CostCNY) || item.FinalPrice <= 0 {
			return false
		}
	}
	return true
}

func mustMarshalSheinPricingReview(review *sheinpub.PricingReview) string {
	data, err := json.Marshal(review)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeSheinPricingCacheEntry(entry *sheinpub.SheinResolutionCacheEntry) *sheinpub.PricingReview {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var review sheinpub.PricingReview
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &review); err != nil {
		return nil
	}
	return cloneSheinPricingReview(&review)
}

func cloneSheinPricingReview(review *sheinpub.PricingReview) *sheinpub.PricingReview {
	if review == nil {
		return nil
	}
	cloned := *review
	if review.RuleSnapshot != nil {
		rule := *review.RuleSnapshot
		cloned.RuleSnapshot = &rule
	}
	if len(review.SKUPrices) > 0 {
		cloned.SKUPrices = append([]sheinpub.SKUPriceReview(nil), review.SKUPrices...)
	}
	if len(review.ManualOverrides) > 0 {
		cloned.ManualOverrides = clonePriceOverrides(review.ManualOverrides)
	}
	if len(review.MissingPriceSKUs) > 0 {
		cloned.MissingPriceSKUs = append([]string(nil), review.MissingPriceSKUs...)
	}
	if review.UpdatedAt != nil {
		updatedAt := *review.UpdatedAt
		cloned.UpdatedAt = &updatedAt
	}
	if review.Cache != nil {
		cloned.Cache = sheinpub.CloneResolutionCacheInfo(review.Cache)
	}
	return &cloned
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
		ShortKey:  sheinPricingShortKey(key),
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
		"store_id":  sheinPricingStoreID(req),
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
			return strings.Join(sheinPricingProductIdentity(pkg), ",")
		}(),
		"sku_facts": func() string {
			if pkg == nil {
				return ""
			}
			return strings.Join(sortedSheinPricingSKUFacts(pkg, sheinpub.PricingRule{}), ",")
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
