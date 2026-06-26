package publishing

import "strings"

// DeriveSubmitSupplierCode resolves the SHEIN submit supplier code from product and SKU identifiers.
func DeriveSubmitSupplierCode(productSupplierCode string, supplierSKUs []string) string {
	if value := strings.TrimSpace(productSupplierCode); value != "" && !looksLikeRawBaseSupplierCode(value) {
		return value
	}
	for _, supplierSKU := range supplierSKUs {
		if value := deriveSubmitSupplierCodeFromSKU(supplierSKU); value != "" {
			return value
		}
	}
	return strings.TrimSpace(productSupplierCode)
}

func deriveSubmitSupplierCodeFromSKU(supplierSKU string) string {
	supplierSKU = strings.TrimSpace(strings.ToUpper(supplierSKU))
	if supplierSKU == "" {
		return ""
	}
	parts := strings.Split(supplierSKU, "-")
	if len(parts) < 2 {
		return supplierSKU
	}
	styleSuffix := normalizeSubmitStyleSuffix(parts[len(parts)-1])
	if styleSuffix == "" {
		return supplierSKU
	}
	baseSKU := strings.TrimSpace(parts[0])
	if baseSKU == "" {
		return ""
	}
	return baseSKU + "-" + styleSuffix
}

func normalizeSubmitStyleSuffix(value string) string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
}

func looksLikeRawBaseSupplierCode(value string) bool {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return false
	}
	parts := strings.Split(value, "-")
	return len(parts) == 1
}
