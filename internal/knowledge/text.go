package knowledge

import (
	"strings"
	"unicode/utf8"
)

func NormalizeText(text, warning string) ParseResult {
	if !utf8.ValidString(text) {
		return ParseResult{Failure: "INVALID_EXTRACTED_TEXT"}
	}
	text = strings.ReplaceAll(text, "\x00", "")
	if len(text) > MaxTextBytes {
		text = text[:MaxTextBytes]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		warning = "TEXT_TRUNCATED"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ParseResult{Failure: "NO_EXTRACTABLE_TEXT"}
	}
	return ParseResult{Text: text, Warning: warning}
}
