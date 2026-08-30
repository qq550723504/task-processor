package publishing

import (
	"regexp"
	"strconv"
	"strings"
)

var temuProductNameAllowedPattern = regexp.MustCompile(`[^a-zA-Z0-9\s\-\+\=\(\)\[\]\.\,\:\/"'&%@]+`)

type ProductNameSanitizationResult struct {
	Name       string
	Violations []string
}

// SanitizeProductName applies the TEMU pre-submission name constraints. It
// reports the same user-facing violation categories as the historical
// validator while keeping optimization and length truncation in the caller.
func SanitizeProductName(name string) ProductNameSanitizationResult {
	var violations []string
	cleaned := name

	for _, char := range []string{"~", "!", "*", "$", "?", "_", "~", "{", "}", "#", "<", ">", "|", "*", ";", "^", "¬", "¦"} {
		if strings.Contains(cleaned, char) {
			violations = append(violations, "包含不支持的装饰字符: "+char)
			cleaned = strings.ReplaceAll(cleaned, char, "")
		}
	}

	var highASCII, chinese bool
	var converted strings.Builder
	for _, r := range cleaned {
		switch {
		case r >= 0x4e00 && r <= 0x9fff:
			chinese = true
		case r > 127:
			highASCII = true
			switch r {
			case '®':
				converted.WriteString("(R)")
			case '©':
				converted.WriteString("(C)")
			case '™':
				converted.WriteString("(TM)")
			}
		default:
			converted.WriteRune(r)
		}
	}
	if chinese {
		violations = append(violations, "包含中文字符（已移除）")
	}
	if highASCII {
		violations = append(violations, "包含高ASCII字符（已转换或移除）")
	}

	cleaned = converted.String()
	if temuProductNameAllowedPattern.MatchString(cleaned) {
		violations = append(violations, "包含不支持的字符")
		cleaned = temuProductNameAllowedPattern.ReplaceAllString(cleaned, " ")
	}
	cleaned = NormalizeProductSubmissionName(cleaned)
	if len(cleaned) > 500 {
		violations = append(violations, "超过500字符限制: "+strconv.Itoa(len(cleaned)))
	}
	return ProductNameSanitizationResult{Name: cleaned, Violations: violations}
}
