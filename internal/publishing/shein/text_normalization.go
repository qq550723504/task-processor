package shein

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	sheinHTMLTagPattern             = regexp.MustCompile(`<[^>]*>`)
	sheinEmojiPattern               = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]`)
	sheinSpecialCharPattern         = regexp.MustCompile(`[^\p{L}\p{N}\s.,!?()-]`)
	sheinWhitespacePattern          = regexp.MustCompile(`\s+`)
	sheinAttributeSpecialCharRegex  = regexp.MustCompile(`[",;:()\[\]{}'"<>|\\/*?+\-=!@#$%^&~【】！？。，、；：""''（）《》〈〉「」『』〔〕［］｛｝…—–‚„†‡•‰‹›€™` + "`" + `]`)
	sheinAttributeInchRegex         = regexp.MustCompile(`(\d+(?:\.\d+)?)"`)
	sheinAttributeFtRegex           = regexp.MustCompile(`(\d+(?:\.\d+)?)'`)
	sheinAttributeReplacementTokens = map[string]string{
		`,`: " ", ` x `: " by ", ` X `: " by ", `×`: " by ", `&`: " and ",
		`+`: " ", `-`: " ", `_`: " ", `/`: " or ", `\\`: " ", `|`: " or ",
		`<`: " ", `>`: " ", `=`: " equals ", `%`: " percent ", `#`: " ",
		`@`: " ", `$`: " dollar ", `!`: " ", `?`: " ", `*`: " ", `~`: " ",
		"`": " ", `(`: " ", `)`: " ", `[`: " ", `]`: " ", `{`: " ", `}`: " ",
		`"`: " ", `'`: " ", `【`: " ", `】`: " ", `！`: " ", `？`: " ",
		`。`: " ", `，`: " ", `、`: " ", `；`: " ", `：`: " ", `（`: " ",
		`）`: " ", `《`: " ", `》`: " ", `〈`: " ", `〉`: " ", `「`: " ",
		`」`: " ", `『`: " ", `』`: " ", `〔`: " ", `〕`: " ", `［`: " ",
		`］`: " ", `｛`: " ", `｝`: " ", `…`: " ", `—`: " ", `–`: " ",
		"\u201C": " ", "\u201D": " ", "\u2018": " ", "\u2019": " ",
	}
)

func normalizeSheinContentText(text string) string {
	text = sheinHTMLTagPattern.ReplaceAllString(text, "")
	text = sheinEmojiPattern.ReplaceAllString(text, "")
	text = sheinSpecialCharPattern.ReplaceAllString(text, "")
	text = sheinWhitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func sanitizeSheinAttributeText(value string) string {
	if value == "" {
		return value
	}
	cleaned := strings.TrimSpace(value)
	if isOnlySheinAttributeSpecialChars(cleaned) {
		return "Custom Value"
	}
	cleaned = sheinAttributeInchRegex.ReplaceAllString(cleaned, "$1 inch")
	cleaned = sheinAttributeFtRegex.ReplaceAllString(cleaned, "$1 ft")
	for pattern, replacement := range sheinAttributeReplacementTokens {
		cleaned = strings.ReplaceAll(cleaned, pattern, replacement)
	}
	cleaned = sheinAttributeSpecialCharRegex.ReplaceAllString(cleaned, "")
	cleaned = removeRemainingSheinAttributeSpecialChars(cleaned)
	cleaned = sheinWhitespacePattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "Custom Value"
	}
	return cleaned
}

func isValidSheinAttributeText(value string) bool {
	if value == "" || strings.TrimSpace(value) == "" || len(value) > 100 {
		return false
	}
	return !sheinAttributeSpecialCharRegex.MatchString(value)
}

func isOnlySheinAttributeSpecialChars(value string) bool {
	if value == "" {
		return false
	}
	cleaned := sheinAttributeSpecialCharRegex.ReplaceAllString(value, "")
	return strings.TrimSpace(cleaned) == ""
}

func removeRemainingSheinAttributeSpecialChars(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '.' {
			return r
		}
		if r == '-' || r == '_' {
			return ' '
		}
		return -1
	}, text)
}
