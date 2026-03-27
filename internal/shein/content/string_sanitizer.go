package content

import (
	"regexp"
	"strings"
	"unicode"
)

// StringSanitizer 字符串清理器
type StringSanitizer struct {
	specialCharsRegex *regexp.Regexp
	multiSpaceRegex   *regexp.Regexp
	inchRegex         *regexp.Regexp
	ftRegex           *regexp.Regexp
}

// NewStringSanitizer 创建新的字符串清理器
func NewStringSanitizer() *StringSanitizer {
	return &StringSanitizer{
		specialCharsRegex: regexp.MustCompile(`[",;:()\[\]{}'"<>|\\/*?+\-=!@#$%^&~【】！？。，、；：""''（）《》〈〉「」『』〔〕［］｛｝…—–‚„†‡•‰‹›€™` + "`" + `]`),
		multiSpaceRegex:   regexp.MustCompile(`\s+`),
		inchRegex:         regexp.MustCompile(`(\d+(?:\.\d+)?)"`),
		ftRegex:           regexp.MustCompile(`(\d+(?:\.\d+)?)'`),
	}
}

// SanitizeForSheinAttribute 为Shein属性值清理字符串
func (s *StringSanitizer) SanitizeForSheinAttribute(value string) string {
	if value == "" {
		return value
	}

	cleaned := strings.TrimSpace(value)

	if s.isOnlySpecialChars(cleaned) {
		return "Custom Value"
	}

	cleaned = s.replaceCommonPatterns(cleaned)
	cleaned = s.specialCharsRegex.ReplaceAllString(cleaned, "")
	cleaned = s.removeRemainingSpecialChars(cleaned)
	cleaned = s.multiSpaceRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "Custom Value"
	}

	return cleaned
}

func (s *StringSanitizer) replaceCommonPatterns(value string) string {
	result := value

	result = s.inchRegex.ReplaceAllString(result, "$1 inch")
	result = s.ftRegex.ReplaceAllString(result, "$1 ft")

	basicPatterns := map[string]string{
		`,`:   " ",
		` x `: " by ",
		` X `: " by ",
		`×`:   " by ",
		`&`:   " and ",
		`+`:   " ",
		`-`:   " ",
		`_`:   " ",
		`/`:   " or ",
		`\\`:  " ",
		`|`:   " or ",
		`<`:   " ",
		`>`:   " ",
		`=`:   " equals ",
		`%`:   " percent ",
		`#`:   " ",
		`@`:   " ",
		`$`:   " dollar ",
		`!`:   " ",
		`?`:   " ",
		`*`:   " ",
		`~`:   " ",
		"`":   " ",
	}
	bracketPatterns := map[string]string{
		`(`: " ", `)`: " ", `[`: " ", `]`: " ", `{`: " ", `}`: " ",
	}
	quotePatterns := map[string]string{
		`"`: " ", `'`: " ",
	}
	chinesePatterns := map[string]string{
		`【`: " ", `】`: " ", `！`: " ", `？`: " ", `。`: " ", `，`: " ",
		`、`: " ", `；`: " ", `：`: " ", `（`: " ", `）`: " ", `《`: " ",
		`》`: " ", `〈`: " ", `〉`: " ", `「`: " ", `」`: " ", `『`: " ",
		`』`: " ", `〔`: " ", `〕`: " ", `［`: " ", `］`: " ", `｛`: " ",
		`｝`: " ", `…`: " ", `—`: " ", `–`: " ",
	}
	chineseQuotePatterns := map[string]string{
		"\u201C": " ", "\u201D": " ", "\u2018": " ", "\u2019": " ",
	}

	for _, patterns := range []map[string]string{basicPatterns, bracketPatterns, quotePatterns, chinesePatterns, chineseQuotePatterns} {
		for pattern, replacement := range patterns {
			result = strings.ReplaceAll(result, pattern, replacement)
		}
	}

	return result
}

func (s *StringSanitizer) isOnlySpecialChars(value string) bool {
	if value == "" {
		return false
	}
	cleaned := s.specialCharsRegex.ReplaceAllString(value, "")
	return strings.TrimSpace(cleaned) == ""
}

// SanitizeForSheinTitle 为Shein标题清理字符串（更宽松的规则）
func (s *StringSanitizer) SanitizeForSheinTitle(title string) string {
	if title == "" {
		return title
	}
	cleaned := strings.TrimSpace(title)
	dangerousChars := regexp.MustCompile(`['"<>|\\]`)
	cleaned = dangerousChars.ReplaceAllString(cleaned, "")
	cleaned = s.multiSpaceRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "Product Title"
	}
	return cleaned
}

// IsValidForSheinAttribute 检查字符串是否适合作为Shein属性值
func (s *StringSanitizer) IsValidForSheinAttribute(value string) bool {
	if value == "" {
		return false
	}
	if s.specialCharsRegex.MatchString(value) {
		return false
	}
	if strings.TrimSpace(value) == "" {
		return false
	}
	if len(value) > 100 {
		return false
	}
	return true
}

// GetSanitizationSuggestion 获取清理建议
func (s *StringSanitizer) GetSanitizationSuggestion(original string) map[string]string {
	sanitized := s.SanitizeForSheinAttribute(original)
	isValid := "false"
	if s.IsValidForSheinAttribute(sanitized) {
		isValid = "true"
	}
	changes := "无变化"
	if original != sanitized {
		changes = "已清理特殊字符"
	}
	return map[string]string{
		"original":  original,
		"sanitized": sanitized,
		"is_valid":  isValid,
		"changes":   changes,
	}
}

func (s *StringSanitizer) removeRemainingSpecialChars(text string) string {
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

// RemoveUnicodeControlChars 移除Unicode控制字符
func (s *StringSanitizer) RemoveUnicodeControlChars(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, text)
}

// TruncateErrorMessage 截断并清理错误信息
func (s *StringSanitizer) TruncateErrorMessage(errorMsg string, maxBytes int) string {
	if errorMsg == "" {
		return errorMsg
	}
	cleaned := s.RemoveUnicodeControlChars(errorMsg)
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	cleaned = strings.ReplaceAll(cleaned, "\ufffd", "")
	if !isValidUTF8(cleaned) {
		cleaned = toValidUTF8(cleaned)
	}
	if len(cleaned) <= maxBytes {
		return cleaned
	}
	truncated := truncateUTF8(cleaned, maxBytes-10)
	if len(cleaned) > maxBytes {
		truncated += "...[截断]"
	}
	return truncated
}

func isValidUTF8(s string) bool {
	return strings.ToValidUTF8(s, "") == s
}

func toValidUTF8(s string) string {
	return strings.ToValidUTF8(s, "?")
}

func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if (s[i] & 0x80) == 0 {
			return s[:i]
		}
		if (s[i] & 0xC0) == 0xC0 {
			return s[:i]
		}
	}
	return ""
}

// 全局默认实例
var DefaultStringSanitizer = NewStringSanitizer()

// SanitizeForSheinAttribute 便捷函数
func SanitizeForSheinAttribute(value string) string {
	return DefaultStringSanitizer.SanitizeForSheinAttribute(value)
}

// SanitizeForSheinTitle 便捷函数
func SanitizeForSheinTitle(title string) string {
	return DefaultStringSanitizer.SanitizeForSheinTitle(title)
}

// IsValidForSheinAttribute 便捷函数
func IsValidForSheinAttribute(value string) bool {
	return DefaultStringSanitizer.IsValidForSheinAttribute(value)
}

// TruncateErrorMessage 截断错误信息的便捷函数（默认400字节）
func TruncateErrorMessage(errorMsg string) string {
	return DefaultStringSanitizer.TruncateErrorMessage(errorMsg, 400)
}

