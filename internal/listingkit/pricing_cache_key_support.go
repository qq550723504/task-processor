package listingkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func sheinPricingCacheKey(req *sheinpub.BuildRequest, pkg *sheinpub.Package, rule sheinpub.PricingRule) string {
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
		"sku_facts":        sortedSheinPricingSKUFacts(pkg, rule),
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

func sortedSheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) []string {
	facts := sheinPricingSKUFacts(pkg, rule)
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
	facts := sheinPricingSKUFacts(pkg, sheinpub.PricingRule{})
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

func sheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) map[string]sheinPricingSKUFact {
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
				Currency:  normalizeSheinReviewCurrency(existingSheinDraftCurrency(sku, ""), rule),
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
