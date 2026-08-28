package publishing

import (
	"regexp"
	"strings"
)

var (
	rightAnyParenthesisSpacingPattern = regexp.MustCompile(`\)(\S)`)
	commaSpacingPattern               = regexp.MustCompile(`\s+,`)
	punctuationSpacingPattern         = regexp.MustCompile(`\s+([.!?;:])`)
)

// NormalizeProductSubmissionName applies the final TEMU name formatting pass
// used immediately before a product submission.
func NormalizeProductSubmissionName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	name = leftParenthesisSpacingPattern.ReplaceAllString(name, "$1 (")
	name = rightAnyParenthesisSpacingPattern.ReplaceAllString(name, ") $1")
	name = commaSpacingPattern.ReplaceAllString(name, ",")
	name = punctuationSpacingPattern.ReplaceAllString(name, "$1")
	return strings.Join(strings.Fields(name), " ")
}
