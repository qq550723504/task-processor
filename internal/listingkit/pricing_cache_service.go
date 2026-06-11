package listingkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) rememberSheinSubmittedPricing(task *Task, action string) {
	if s == nil || s.sheinResolutionCacheStore == nil || task == nil || task.Result == nil || task.Result.Shein == nil || strings.TrimSpace(action) != "publish" {
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
	key := sheinPricingCacheKey(req, task.Result.Shein)
	if key == "" {
		return
	}
	now := time.Now()
	attachPricingCacheInfo(review, "manual_cache", key, true, sheinpub.ResolutionCacheHitSourcePublishRemembered, "stored", 0, &now)
	task.Result.Shein.Pricing = review
	_ = s.sheinResolutionCacheStore.SaveResolutionCache(context.Background(), &sheinpub.SheinResolutionCacheEntry{
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
		"sku_facts":          strings.Join(sortedSheinPricingSKUFacts(task.Result.Shein), ","),
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
	if s.sheinResolutionCacheStore == nil {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "no_resolution_cache_store"})
		return nil
	}
	key := sheinPricingCacheKey(buildReq, pkg)
	if key == "" {
		logPricingCacheEvent("skip", buildReq, pkg, nil, logrus.Fields{"reason": "empty_cache_key"})
		return nil
	}
	entry, err := s.sheinResolutionCacheStore.GetResolutionCache(context.Background(), sheinpub.ResolutionCacheKindPricing, sheinPricingStoreID(buildReq), key)
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
	if s == nil || s.sheinResolutionCacheStore == nil {
		return nil
	}
	key := sheinPricingCacheKey(req, pkg)
	if key == "" {
		return nil
	}
	if pkg != nil && pkg.Pricing != nil {
		pkg.Pricing.Cache = nil
	}
	return s.sheinResolutionCacheStore.DeleteResolutionCache(context.Background(), sheinpub.ResolutionCacheKindPricing, sheinPricingStoreID(req), key)
}

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
	current := sheinPricingSKUFacts(pkg)
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

func sheinPricingCacheKey(req *sheinpub.BuildRequest, pkg *sheinpub.Package) string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return ""
	}
	payload := map[string]any{
		"version":          1,
		"store_id":         sheinPricingStoreID(req),
		"category_id":      pkg.CategoryID,
		"category_id_list": append([]int(nil), pkg.CategoryIDList...),
		"category_path":    normalizedTextList(pkg.CategoryPath),
		"product_identity": sheinPricingProductIdentity(pkg),
		"sku_facts":        sortedSheinPricingSKUFacts(pkg),
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sheinPricingSourceIdentity(pkg *sheinpub.Package) string {
	payload := map[string]any{
		"category_path":    normalizedTextList(pkg.CategoryPath),
		"product_identity": sheinPricingProductIdentity(pkg),
		"sku_aliases":      sortedSheinPricingSKUAliases(pkg),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func sortedSheinPricingSKUFacts(pkg *sheinpub.Package) []string {
	facts := sheinPricingSKUFacts(pkg)
	if len(facts) == 0 {
		return nil
	}
	result := make([]string, 0, len(facts))
	for alias, fact := range facts {
		result = append(result, alias+"|cost="+fact.CostPrice+"|currency="+fact.Currency)
	}
	sort.Strings(result)
	return result
}

func sortedSheinPricingSKUAliases(pkg *sheinpub.Package) []string {
	facts := sheinPricingSKUFacts(pkg)
	if len(facts) == 0 {
		return nil
	}
	result := make([]string, 0, len(facts))
	for alias := range facts {
		result = append(result, alias)
	}
	sort.Strings(result)
	return result
}

type sheinPricingSKUFact struct {
	CostPrice string
	Currency  string
}

func sheinPricingSKUFacts(pkg *sheinpub.Package) map[string]sheinPricingSKUFact {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	result := map[string]sheinPricingSKUFact{}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			alias := sheinPricingDraftSKUKey(&sku)
			if alias == "" {
				continue
			}
			result[alias] = sheinPricingSKUFact{
				CostPrice: formatMoney(parseMoney(sku.CostPrice)),
				Currency:  strings.ToUpper(strings.TrimSpace(existingSheinDraftCurrency(sku, ""))),
			}
		}
	}
	return result
}

func sheinPricingProductIdentity(pkg *sheinpub.Package) []string {
	return sheinpub.StablePricingPackageIdentity(pkg)
}

func lookupSheinAttributeValue(attrs []common.Attribute, name string) string {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, item := range attrs {
		if strings.ToLower(strings.TrimSpace(item.Name)) == target {
			return item.Value
		}
	}
	return ""
}

func normalizedTextList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func sheinPricingSKUAlias(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "-V"); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	parts := strings.Split(value, "-")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if looksLikePricingRequestToken(part) {
			continue
		}
		filtered = append(filtered, part)
	}
	alias := strings.Join(filtered, "-")
	if prefix, ok := trimStyleSuffix(alias); ok {
		return prefix
	}
	return alias
}

func sheinPricingDraftSKUKey(sku *sheinpub.SKUDraft) string {
	if sku == nil {
		return ""
	}
	if source := strings.TrimSpace(sku.Attributes["source_sds_sku"]); source != "" {
		return sheinPricingSKUAlias(source)
	}
	return sheinPricingSKUAlias(sku.SupplierSKU)
}

func sheinPricingReviewSKUKey(item sheinpub.SKUPriceReview) string {
	if key := sheinPricingSKUAlias(item.SupplierSKU); key != "" {
		return key
	}
	return sheinPricingSKUAlias(item.SupplierCode)
}

func trimStyleSuffix(value string) (string, bool) {
	index := strings.LastIndex(value, "-")
	if index <= 0 {
		return "", false
	}
	suffix := strings.TrimSpace(value[index+1:])
	if len(suffix) != 8 {
		return "", false
	}
	for _, r := range suffix {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return "", false
		}
	}
	prefix := strings.TrimSpace(value[:index])
	if prefix == "" || !strings.ContainsAny(prefix, "0123456789") {
		return "", false
	}
	return prefix, true
}

func looksLikePricingRequestToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) < 6 || len(token) > 9 || !strings.HasPrefix(token, "R") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
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
			return strings.Join(sortedSheinPricingSKUFacts(pkg), ",")
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

func sheinPricingStoreID(req *sheinpub.BuildRequest) string {
	if req == nil || req.SheinStoreID == 0 {
		return ""
	}
	return strconv.FormatInt(req.SheinStoreID, 10)
}

func sheinPricingShortKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}
