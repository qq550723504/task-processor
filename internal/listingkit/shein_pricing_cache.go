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
	req := buildSheinPublishRequest(task.Request)
	review := normalizePublishedSheinPricingReview(task.Result.Shein)
	if review == nil {
		review = buildSheinDraftBackedPricingReview(task.Result.Shein, s.currentSheinPricingRule(), nil)
		review = normalizePublishedSheinPricingReview(&sheinpub.Package{
			RequestDraft: task.Result.Shein.RequestDraft,
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
	attachPricingCacheInfo(review, "manual_cache", key, true, 0, nil)
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
	})
}

func (s *service) loadSheinPricingCache(req *GenerateRequest, pkg *sheinpub.Package) *sheinpub.PricingReview {
	if s == nil || s.sheinResolutionCacheStore == nil || pkg == nil {
		return nil
	}
	buildReq := buildSheinPublishRequest(req)
	key := sheinPricingCacheKey(buildReq, pkg)
	if key == "" {
		return nil
	}
	entry, err := s.sheinResolutionCacheStore.GetResolutionCache(context.Background(), sheinpub.ResolutionCacheKindPricing, sheinPricingStoreID(buildReq), key)
	if err != nil || entry == nil {
		return nil
	}
	review := decodeSheinPricingCacheEntry(entry)
	if !sheinPricingReviewApplicable(pkg, review) {
		return nil
	}
	attachPricingCacheInfo(review, cacheEntrySourceLabel(entry), entry.CacheKey, entry.Manual, entry.HitCount, &entry.UpdatedAt)
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
	if pkg == nil || pkg.RequestDraft == nil || review == nil || !review.Ready || len(review.SKUPrices) == 0 {
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
	if pkg == nil || pkg.RequestDraft == nil {
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
	if pkg == nil || pkg.RequestDraft == nil {
		return nil
	}
	result := map[string]sheinPricingSKUFact{}
	for _, skc := range pkg.RequestDraft.SKCList {
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
	hitCount int,
	updatedAt *time.Time,
) {
	if review == nil {
		return
	}
	info := &sheinpub.ResolutionCacheInfo{
		Status:    pricingCacheStatusForSource(source),
		Source:    source,
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

func pricingCacheStatusForSource(source string) string {
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

func logPricingCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {
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
