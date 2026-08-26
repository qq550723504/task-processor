package publishing

import (
	"regexp"
	"strings"
)

var (
	leftParenthesisSpacingPattern  = regexp.MustCompile(`(\S)\(`)
	rightParenthesisSpacingPattern = regexp.MustCompile(`\)([a-zA-Z0-9])`)
	whitespacePattern              = regexp.MustCompile(`\s+`)
)

// NormalizeParenthesesSpacing applies the TEMU product-name spacing rule.
// Punctuation adjacency is preserved; only a parenthesis next to a word and
// repeated whitespace are normalized.
func NormalizeParenthesesSpacing(name string) string {
	name = leftParenthesisSpacingPattern.ReplaceAllString(name, "$1 (")
	name = rightParenthesisSpacingPattern.ReplaceAllString(name, ") $1")
	name = whitespacePattern.ReplaceAllString(name, " ")
	return strings.TrimSpace(name)
}
