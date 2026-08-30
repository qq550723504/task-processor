package publishing

import (
	"fmt"
	"strings"
)

const temuProductDescriptionMaxLength = 10_000

type ProductDescriptionSanitizationResult struct {
	Description string
	Violations  []string
}

// SanitizeProductDescription applies the TEMU description-cleaning behavior
// used before publishing without depending on the historical TEMU runtime.
func SanitizeProductDescription(description string) ProductDescriptionSanitizationResult {
	cleaned := whitespacePattern.ReplaceAllString(strings.TrimSpace(description), " ")

	var asciiOnly strings.Builder
	for _, r := range cleaned {
		if r <= 127 {
			asciiOnly.WriteRune(r)
		}
	}
	cleaned = asciiOnly.String()

	var violations []string
	if len(cleaned) > temuProductDescriptionMaxLength {
		violations = append(violations, fmt.Sprintf("描述长度超过限制: %d > %d字符", len(cleaned), temuProductDescriptionMaxLength))
		cleaned = truncateProductDescription(cleaned, temuProductDescriptionMaxLength)
	}

	return ProductDescriptionSanitizationResult{Description: cleaned, Violations: violations}
}

func truncateProductDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	truncated := description[:maxLength-3]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLength/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}
