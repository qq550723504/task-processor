package shein

import (
	"regexp"
	"strings"
)

var (
	sheinHTMLTagPattern     = regexp.MustCompile(`<[^>]*>`)
	sheinEmojiPattern       = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`)
	sheinSpecialCharPattern = regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()-]`)
	sheinWhitespacePattern  = regexp.MustCompile(`\s+`)
)

func normalizeSheinContentText(text string) string {
	text = sheinHTMLTagPattern.ReplaceAllString(text, "")
	text = sheinEmojiPattern.ReplaceAllString(text, "")
	text = sheinSpecialCharPattern.ReplaceAllString(text, "")
	text = sheinWhitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
