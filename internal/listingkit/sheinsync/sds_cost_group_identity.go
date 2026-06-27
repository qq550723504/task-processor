package sheinsync

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

type SheinSDSCostGroupIdentity struct {
	GroupKey        string
	GroupLabel      string
	SourceCode      string
	SKUCode         string
	VariantLabel    string
	SKUCodes        []string
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
	identities := ResolveSheinSDSVariantCostGroupIdentities(product)
	if len(identities) > 0 {
		return identities[0]
	}
	return SheinSDSCostGroupIdentity{}
}

func ResolveSheinSDSVariantCostGroupIdentities(product SheinSyncedProductRecord) []SheinSDSCostGroupIdentity {
	base := ResolveSheinSDSCostGroupIdentity(product)
	if base.GroupKey == "" || base.SourceCode == "" {
		return nil
	}

	skuInfos := SheinSyncedProductSKUInfos(product)
	variants := make(map[string][]string)
	for _, skuInfo := range skuInfos {
		variantCode := sheinSourceVariantCodeFromSupplierSKU(skuInfo.SupplierSKU)
		if variantCode == "" {
			continue
		}
		variants[variantCode] = appendMissingString(variants[variantCode], skuInfo.SKUCode)
	}
	if len(variants) == 0 {
		for _, skuInfo := range skuInfos {
			variantLabel := strings.TrimSpace(skuInfo.VariantLabel)
			if variantLabel == "" {
				continue
			}
			variants[variantLabel] = appendMissingString(variants[variantLabel], skuInfo.SKUCode)
		}
	}
	if len(variants) == 0 {
		variantLabel := sheinSyncedProductVariantLabel(product)
		if variantLabel == "" {
			return nil
		}
		variants[variantLabel] = SheinSyncedProductSKUCodes(product)
	}

	out := make([]SheinSDSCostGroupIdentity, 0, len(variants))
	for variantLabel, skuCodes := range variants {
		legacyKeys := make([]string, 0, len(base.LegacyGroupKeys)+1+len(skuCodes))
		legacyKeys = append(legacyKeys, base.GroupKey)
		legacyKeys = append(legacyKeys, base.LegacyGroupKeys...)
		for _, skuCode := range skuCodes {
			if skuCode != "" {
				legacyKeys = append(legacyKeys, base.GroupKey+":sku:"+skuCode)
			}
		}
		out = append(out, SheinSDSCostGroupIdentity{
			GroupKey:        base.GroupKey + ":variant:" + sheinSDSVariantKeySuffix(variantLabel),
			GroupLabel:      base.GroupLabel + " / " + variantLabel,
			SourceCode:      base.SourceCode,
			SKUCode:         variantLabel,
			VariantLabel:    variantLabel,
			SKUCodes:        append([]string(nil), skuCodes...),
			LegacyGroupKeys: legacyKeys,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].VariantLabel != out[j].VariantLabel {
			return out[i].VariantLabel < out[j].VariantLabel
		}
		return out[i].GroupKey < out[j].GroupKey
	})
	return out
}

func SheinSyncedProductSKUCodes(product SheinSyncedProductRecord) []string {
	skuInfos := SheinSyncedProductSKUInfos(product)
	out := make([]string, 0, len(skuInfos))
	for _, skuInfo := range skuInfos {
		out = appendMissingString(out, skuInfo.SKUCode)
	}
	return out
}

type SheinSyncedProductSKUInfo struct {
	SKUCode      string
	SupplierSKU  string
	VariantLabel string
}

func SheinSyncedProductSKUInfos(product SheinSyncedProductRecord) []SheinSyncedProductSKUInfo {
	if strings.TrimSpace(product.SiteSnapshot) == "" {
		return nil
	}

	var payload struct {
		SKUCodes []string `json:"sku_codes"`
		SKUInfo  []struct {
			SKUCode      string `json:"sku_code"`
			SupplierSKU  string `json:"supplier_sku"`
			VariantLabel string `json:"variant_label"`
		} `json:"sku_info"`
	}
	if err := json.Unmarshal([]byte(product.SiteSnapshot), &payload); err != nil {
		return nil
	}

	out := make([]SheinSyncedProductSKUInfo, 0, len(payload.SKUCodes)+len(payload.SKUInfo))
	seen := map[string]struct{}{}
	appendInfo := func(skuCode, supplierSKU, variantLabel string) {
		info := SheinSyncedProductSKUInfo{
			SKUCode:      strings.ToUpper(strings.TrimSpace(skuCode)),
			SupplierSKU:  strings.ToUpper(strings.TrimSpace(supplierSKU)),
			VariantLabel: strings.TrimSpace(variantLabel),
		}
		if info.SKUCode == "" && info.SupplierSKU == "" && info.VariantLabel == "" {
			return
		}
		key := info.SKUCode + "\x00" + info.SupplierSKU + "\x00" + strings.ToUpper(info.VariantLabel)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, info)
	}
	for _, code := range payload.SKUCodes {
		appendInfo(code, "", "")
	}
	for _, sku := range payload.SKUInfo {
		appendInfo(sku.SKUCode, sku.SupplierSKU, sku.VariantLabel)
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
	if normalized != "" && sheinSDSVariantKeySafe(normalized) {
		return normalized
	}
	sum := sha1.Sum([]byte(normalized))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:12])
}

func sheinSDSVariantKeySafe(value string) bool {
	if len(value) > 48 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			continue
		default:
			return false
		}
	}
	return true
}

// sheinSourceVariantCodeFromSupplierSKU recovers the SDS variant product code used by cost metadata.
func sheinSourceVariantCodeFromSupplierSKU(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	if idx := strings.Index(normalized, "-"); idx > 0 {
		return strings.TrimSpace(normalized[:idx])
	}
	return normalized
}

func appendMissingString(existing []string, value string) []string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return existing
	}
	for _, item := range existing {
		if item == value {
			return existing
		}
	}
	return append(existing, value)
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
