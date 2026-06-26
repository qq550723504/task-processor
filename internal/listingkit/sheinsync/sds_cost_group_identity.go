package sheinsync

import "strings"

type SheinSDSCostGroupIdentity struct {
	GroupKey        string
	GroupLabel      string
	SourceCode      string
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
