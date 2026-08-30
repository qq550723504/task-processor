package publishing

import "strings"

// RemoveBrandFromText removes TEMU brand-name variants and normalizes the
// resulting text for product submission.
func RemoveBrandFromText(text, brandName string) string {
	if text == "" || brandName == "" {
		return text
	}

	brandVariants := []string{
		brandName,
		strings.ToUpper(brandName),
		strings.ToLower(brandName),
	}

	result := text
	for _, variant := range brandVariants {
		result = strings.ReplaceAll(result, variant+" ", "")
		result = strings.ReplaceAll(result, " "+variant, "")
		result = strings.ReplaceAll(result, variant+",", "")
		result = strings.ReplaceAll(result, ","+variant, "")
		result = strings.ReplaceAll(result, variant+"-", "")
		result = strings.ReplaceAll(result, "-"+variant, "")
		result = strings.ReplaceAll(result, variant+"'s", "")
		result = strings.ReplaceAll(result, variant, "")
	}

	return NormalizeProductSubmissionName(result)
}
