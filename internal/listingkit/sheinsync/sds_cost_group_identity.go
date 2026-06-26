package sheinsync

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type SheinSDSCostGroupIdentity struct {
	GroupKey        string
	GroupLabel      string
	SourceCode      string
	SKUCode         string
	VariantLabel    string
	LegacyGroupKeys []string
}

func ResolveSheinSDSCostGroupIdentity(product SheinSyncedProductRecord) SheinSDSCostGroupIdentity {
	supplierCode := strings.TrimSpace(product.SupplierCode)
	if supplierCode == "" {
		return SheinSDSCostGroupIdentity{}
	}

	sourceCode := sheinSourceSDSCode(supplierCode)
	suffix, hasSuffix := sheinSDSStyleSuffix(supplierCode)
	if sourceCode != "" {
		identity := SheinSDSCostGroupIdentity{
			GroupKey:   "source:" + sourceCode,
			GroupLabel: sourceCode,
			SourceCode: sourceCode,
		}
		if hasSuffix {
			identity.LegacyGroupKeys = append(identity.LegacyGroupKeys, "style:"+suffix)
		}
		identity.LegacyGroupKeys = append(identity.LegacyGroupKeys, "supplier:"+supplierCode)
		return identity
	}
	if hasSuffix {
		return SheinSDSCostGroupIdentity{
			GroupKey:   "style:" + suffix,
			GroupLabel: suffix,
		}
	}
	return SheinSDSCostGroupIdentity{
		GroupKey:   "supplier:" + supplierCode,
		GroupLabel: supplierCode,
	}
}

func ResolveSheinSDSSKUCostGroupIdentities(product SheinSyncedProductRecord) []SheinSDSCostGroupIdentity {
	base := ResolveSheinSDSCostGroupIdentity(product)
	if base.GroupKey == "" || base.SourceCode == "" {
		return nil
	}

	skuCodes := SheinSyncedProductSKUCodes(product)
	if len(skuCodes) == 0 {
		return nil
	}

	out := make([]SheinSDSCostGroupIdentity, 0, len(skuCodes))
	for _, skuCode := range skuCodes {
		legacyKeys := make([]string, 0, len(base.LegacyGroupKeys)+1)
		legacyKeys = append(legacyKeys, base.GroupKey)
		legacyKeys = append(legacyKeys, base.LegacyGroupKeys...)
		out = append(out, SheinSDSCostGroupIdentity{
			GroupKey:        base.GroupKey + ":sku:" + skuCode,
			GroupLabel:      base.GroupLabel + " / " + skuCode,
			SourceCode:      base.SourceCode,
			SKUCode:         skuCode,
			LegacyGroupKeys: legacyKeys,
		})
	}
	return out
}

func ResolveSheinSDSVariantCostGroupIdentity(product SheinSyncedProductRecord) SheinSDSCostGroupIdentity {
	base := ResolveSheinSDSCostGroupIdentity(product)
	if base.GroupKey == "" || base.SourceCode == "" {
		return SheinSDSCostGroupIdentity{}
	}

	variantLabel := sheinSyncedProductVariantLabel(product)
	if variantLabel == "" {
		return SheinSDSCostGroupIdentity{}
	}

	legacyKeys := make([]string, 0, len(base.LegacyGroupKeys)+1+len(SheinSyncedProductSKUCodes(product)))
	legacyKeys = append(legacyKeys, base.GroupKey)
	legacyKeys = append(legacyKeys, base.LegacyGroupKeys...)
	for _, skuCode := range SheinSyncedProductSKUCodes(product) {
		legacyKeys = append(legacyKeys, base.GroupKey+":sku:"+skuCode)
	}
	return SheinSDSCostGroupIdentity{
		GroupKey:        base.GroupKey + ":variant:" + sheinSDSVariantKeySuffix(variantLabel),
		GroupLabel:      base.GroupLabel + " / " + variantLabel,
		SourceCode:      base.SourceCode,
		SKUCode:         variantLabel,
		VariantLabel:    variantLabel,
		LegacyGroupKeys: legacyKeys,
	}
}

func SheinSyncedProductSKUCodes(product SheinSyncedProductRecord) []string {
	if strings.TrimSpace(product.SiteSnapshot) == "" {
		return nil
	}

	var payload struct {
		SKUCodes []string `json:"sku_codes"`
		SKUInfo  []struct {
			SKUCode string `json:"sku_code"`
		} `json:"sku_info"`
	}
	if err := json.Unmarshal([]byte(product.SiteSnapshot), &payload); err != nil {
		return nil
	}

	out := make([]string, 0, len(payload.SKUCodes)+len(payload.SKUInfo))
	seen := map[string]struct{}{}
	appendCode := func(value string) {
		code := strings.ToUpper(strings.TrimSpace(value))
		if code == "" {
			return
		}
		if _, ok := seen[code]; ok {
			return
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	for _, code := range payload.SKUCodes {
		appendCode(code)
	}
	for _, sku := range payload.SKUInfo {
		appendCode(sku.SKUCode)
	}
	return out
}

func sheinSyncedProductVariantLabel(product SheinSyncedProductRecord) string {
	if label := strings.TrimSpace(product.SaleName); label != "" {
		return label
	}
	if strings.TrimSpace(product.SiteSnapshot) == "" {
		return ""
	}

	var payload struct {
		SaleName string `json:"sale_name"`
	}
	if err := json.Unmarshal([]byte(product.SiteSnapshot), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.SaleName)
}

func sheinSDSVariantKeySuffix(label string) string {
	normalized := strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(label)), " "))
	sum := sha1.Sum([]byte(normalized))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:12])
}

func sheinSourceSDSCode(supplierCode string) string {
	normalized := strings.TrimSpace(supplierCode)
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "-")
	if len(parts) >= 2 {
		suffix := strings.TrimSpace(parts[len(parts)-1])
		if _, ok := sheinSDSStyleSuffix(normalized); ok && suffix != "" {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}
	return normalized
}

func sheinSDSStyleSuffix(supplierCode string) (string, bool) {
	idx := strings.LastIndex(supplierCode, "-")
	if idx < 0 || idx == len(supplierCode)-1 {
		return "", false
	}
	suffix := strings.ToUpper(strings.TrimSpace(supplierCode[idx+1:]))
	if len(suffix) != 8 {
		return "", false
	}
	for _, r := range suffix {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			continue
		default:
			return "", false
		}
	}
	return suffix, true
}
