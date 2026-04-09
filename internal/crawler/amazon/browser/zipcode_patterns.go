package browser

import (
	"regexp"
	"strings"
)

var (
	usZipcodePattern     = regexp.MustCompile(`^\d{5}(?:-\d{4})?$`)
	canadaZipcodePattern = regexp.MustCompile(`^[A-Z]\d[A-Z]\s?\d[A-Z]\d$`)
	japanZipcodePattern  = regexp.MustCompile(`^\d{3}-?\d{4}$`)
	ukZipcodePattern     = regexp.MustCompile(`(?i)^[A-Z]{1,2}\d{1,2}[A-Z]?\s?\d[A-Z]{2}$`)

	extractJapanZipcodePattern  = regexp.MustCompile(`\b\d{3}-\d{4}\b`)
	extractBrazilZipcodePattern = regexp.MustCompile(`\b\d{5}-\d{3}\b`)
	extractUSZipcodePattern     = regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`)
	extractUKZipcodePattern     = regexp.MustCompile(`\b[A-Z]{1,2}\d{1,2}[A-Z]?\s?\d[A-Z]{2}\b`)
	extractCanadaZipcodePattern = regexp.MustCompile(`\b[A-Z]\d[A-Z]\s?\d[A-Z]\d\b`)
	extractCanadaFSAPattern     = regexp.MustCompile(`\b[A-Z]\d[A-Z]\b`)
	extractSimpleZipcodePattern = regexp.MustCompile(`\b\d{4,6}\b`)

	locationWhitespacePattern = regexp.MustCompile(`\s+`)
)

func inferCountryFromZipcodeValue(zipcode string) string {
	normalized := strings.ToUpper(strings.TrimSpace(zipcode))
	if normalized == "" {
		return ""
	}

	switch {
	case usZipcodePattern.MatchString(normalized):
		return "United States"
	case canadaZipcodePattern.MatchString(normalized):
		return "Canada"
	case japanZipcodePattern.MatchString(normalized):
		return "Japan"
	case ukZipcodePattern.MatchString(normalized):
		return "United Kingdom"
	default:
		return ""
	}
}
