package shein

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

// PricingSKUFact is the normalized SKU identity used by SHEIN pricing cache keys.
type PricingSKUFact struct {
	CostPrice string
	Currency  string
}

// PricingCacheKey returns a stable cache key for a SHEIN pricing request.
func PricingCacheKey(req *BuildRequest, pkg *Package, rule PricingRule) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return ""
	}
	payload := map[string]any{
		"version":          1,
		"store_id":         PricingStoreID(req),
		"category_id":      pkg.CategoryID,
		"category_id_list": append([]int(nil), pkg.CategoryIDList...),
		"category_path":    normalizedPricingTextList(pkg.CategoryPath),
		"product_identity": PricingProductIdentity(pkg),
		"sku_facts":        SortedPricingSKUFacts(pkg, rule),
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// PricingSourceIdentity returns a stable human-readable identity for cache diagnostics.
func PricingSourceIdentity(pkg *Package) string {
	payload := map[string]any{
		"category_path":    normalizedPricingTextList(pkg.CategoryPath),
		"product_identity": PricingProductIdentity(pkg),
		"sku_aliases":      SortedPricingSKUAliases(pkg),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func SortedPricingSKUFacts(pkg *Package, rule PricingRule) []string {
	facts := PricingSKUFacts(pkg, rule)
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

func SortedPricingSKUAliases(pkg *Package) []string {
	facts := PricingSKUFacts(pkg, PricingRule{})
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

func PricingSKUFacts(pkg *Package, rule PricingRule) map[string]PricingSKUFact {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	result := map[string]PricingSKUFact{}
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			alias := PricingDraftSKUKey(&sku)
			if alias == "" {
				continue
			}
			result[alias] = PricingSKUFact{
				CostPrice: formatPricingMoney(parsePricingMoney(sku.CostPrice)),
				Currency:  normalizePricingReviewCurrency(existingDraftPricingCurrency(sku, ""), rule),
			}
		}
	}
	return result
}

func PricingProductIdentity(pkg *Package) []string {
	return StablePricingPackageIdentity(pkg)
}

func PricingSKUAlias(value string) string {
	return pricingCacheSKUAlias(value)
}

func PricingDraftSKUKey(sku *SKUDraft) string {
	return pricingCacheDraftSKUKey(sku)
}

func PricingReviewSKUKey(item SKUPriceReview) string {
	return pricingCacheReviewSKUKey(item)
}

func PricingStoreID(req *BuildRequest) string {
	if req == nil || req.SheinStoreID == 0 {
		return ""
	}
	return strconv.FormatInt(req.SheinStoreID, 10)
}

func PricingShortKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:12]
}

func PricingReviewApplicable(pkg *Package, review *PricingReview) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || review == nil || !review.Ready || len(review.SKUPrices) == 0 {
		return false
	}
	current := PricingSKUFacts(pkg, PricingRule{})
	if len(current) == 0 || len(current) != len(review.SKUPrices) {
		return false
	}
	for _, item := range review.SKUPrices {
		key := PricingReviewSKUKey(item)
		fact, ok := current[key]
		if !ok || fact.CostPrice != formatPricingMoney(item.CostCNY) || item.FinalPrice <= 0 {
			return false
		}
	}
	return true
}

func NormalizePublishedPricingReview(pkg *Package) *PricingReview {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	review := ClonePricingReview(pkg.Pricing)
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

func DecodePricingCacheEntry(entry *SheinResolutionCacheEntry) *PricingReview {
	if entry == nil || strings.TrimSpace(entry.ResolutionJSON) == "" {
		return nil
	}
	var review PricingReview
	if err := json.Unmarshal([]byte(entry.ResolutionJSON), &review); err != nil {
		return nil
	}
	return ClonePricingReview(&review)
}

func ClonePricingReview(review *PricingReview) *PricingReview {
	if review == nil {
		return nil
	}
	cloned := *review
	if review.RuleSnapshot != nil {
		rule := *review.RuleSnapshot
		cloned.RuleSnapshot = &rule
	}
	if len(review.SKUPrices) > 0 {
		cloned.SKUPrices = append([]SKUPriceReview(nil), review.SKUPrices...)
	}
	if len(review.ManualOverrides) > 0 {
		cloned.ManualOverrides = clonePricingOverrides(review.ManualOverrides)
	}
	if len(review.MissingPriceSKUs) > 0 {
		cloned.MissingPriceSKUs = append([]string(nil), review.MissingPriceSKUs...)
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

func clonePricingOverrides(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]float64, len(input))
	for sku, price := range input {
		if strings.TrimSpace(sku) != "" && price > 0 {
			out[sku] = price
		}
	}
	return out
}

func normalizedPricingTextList(values []string) []string {
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

func parsePricingMoney(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func formatPricingMoney(value float64) string {
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', 2, 64)
}

func existingDraftPricingCurrency(sku SKUDraft, fallback string) string {
	if value := strings.ToUpper(strings.TrimSpace(sku.Currency)); value != "" {
		return value
	}
	for _, item := range sku.SitePriceList {
		if value := strings.ToUpper(strings.TrimSpace(item.Currency)); value != "" {
			return value
		}
	}
	fallback = strings.ToUpper(strings.TrimSpace(fallback))
	if fallback == "" {
		return "USD"
	}
	return fallback
}

func normalizePricingReviewCurrency(currency string, rule PricingRule) string {
	sourceCurrency := strings.ToUpper(strings.TrimSpace(rule.SourceCurrency))
	if sourceCurrency == "" {
		sourceCurrency = "CNY"
	}
	targetCurrency := strings.ToUpper(strings.TrimSpace(rule.TargetCurrency))
	if targetCurrency == "" {
		targetCurrency = "USD"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" || currency == sourceCurrency {
		return targetCurrency
	}
	return currency
}
