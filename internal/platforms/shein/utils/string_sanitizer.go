package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// StringSanitizer 字符串清理器
type StringSanitizer struct {
	// 特殊字符正则表达式
	specialCharsRegex *regexp.Regexp
	// 多空格正则表达式
	multiSpaceRegex *regexp.Regexp
	// 尺寸模式正则表达式
	inchRegex *regexp.Regexp
	ftRegex   *regexp.Regexp
}

// NewStringSanitizer 创建新的字符串清理器
func NewStringSanitizer() *StringSanitizer {
	return &StringSanitizer{
		// 匹配常见的特殊字符：引号、逗号、分号、冒号、括号等，包括中文特殊字符
		// 特别包含: # () \+-@`~<
		specialCharsRegex: regexp.MustCompile(`[",;:()\[\]{}'"<>|\\/*?+\-=!@#$%^&~【】！？。，、；：""''（）《》〈〉「」『』〔〕［］｛｝…—–‚„†‡•‰‹›€™` + "`" + `]`),
		multiSpaceRegex:   regexp.MustCompile(`\s+`),
		// 匹配数字（包括小数）+引号的模式，如 15.8" -> 15.8 inch
		inchRegex: regexp.MustCompile(`(\d+(?:\.\d+)?)"`),
		// 匹配数字（包括小数）+单引号的模式，如 5.5' -> 5.5 ft
		ftRegex: regexp.MustCompile(`(\d+(?:\.\d+)?)'`),
	}
}

// SanitizeForSheinAttribute 为Shein属性值清理字符串
// 移除或替换不被Shein平台接受的特殊字符
func (s *StringSanitizer) SanitizeForSheinAttribute(value string) string {
	if value == "" {
		return value
	}

	// 1. 去除首尾空格
	cleaned := strings.TrimSpace(value)

	// 2. 检查是否只包含特殊字符，如果是，直接返回默认值
	if s.isOnlySpecialChars(cleaned) {
		return "Custom Value"
	}

	// 3. 替换常见的特殊字符组合
	cleaned = s.replaceCommonPatterns(cleaned)

	// 4. 移除剩余的特殊字符（多次清理确保彻底）
	cleaned = s.specialCharsRegex.ReplaceAllString(cleaned, "")

	// 5. 额外清理：移除任何剩余的Unicode特殊字符
	cleaned = s.removeRemainingSpecialChars(cleaned)

	// 6. 清理多余的空格
	cleaned = s.multiSpaceRegex.ReplaceAllString(cleaned, " ")

	// 7. 再次去除首尾空格
	cleaned = strings.TrimSpace(cleaned)

	// 8. 如果清理后为空，返回安全的默认值
	if cleaned == "" {
		return "Custom Value"
	}

	return cleaned
}

// replaceCommonPatterns 替换常见的特殊字符模式
func (s *StringSanitizer) replaceCommonPatterns(value string) string {
	result := value

	// 1. 首先处理尺寸相关的模式（更精确的匹配）
	// 处理数字+引号的情况（如 13" -> 13 inch）
	result = s.inchRegex.ReplaceAllString(result, "$1 inch")

	// 处理数字+单引号的情况（如 5' -> 5 ft）
	result = s.ftRegex.ReplaceAllString(result, "$1 ft")

	// 2. 分批处理特殊字符，避免键重复
	// 基本符号替换
	basicPatterns := map[string]string{
		`,`:   " ",         // 逗号替换为空格
		` x `: " by ",      // 尺寸分隔符
		` X `: " by ",      // 大写X
		`×`:   " by ",      // 乘号
		`&`:   " and ",     // 与符号
		`+`:   " ",         // 加号替换为空格
		`-`:   " ",         // 连字符替换为空格
		`_`:   " ",         // 下划线替换为空格
		`/`:   " or ",      // 斜杠替换为or
		`\\`:  " ",         // 反斜杠替换为空格
		`|`:   " or ",      // 竖线替换为or
		`<`:   " ",         // 小于号替换为空格
		`>`:   " ",         // 大于号替换为空格
		`=`:   " equals ",  // 等号
		`%`:   " percent ", // 百分号
		`#`:   " ",         // 井号替换为空格
		`@`:   " ",         // at符号替换为空格
		`$`:   " dollar ",  // 美元符号
		`!`:   " ",         // 感叹号
		`?`:   " ",         // 问号
		`*`:   " ",         // 星号
		`~`:   " ",         // 波浪号替换为空格
		"`":   " ",         // 反引号替换为空格
	}

	// 括号类符号（替换为空格）
	bracketPatterns := map[string]string{
		`(`: " ", // 左括号替换为空格
		`)`: " ", // 右括号替换为空格
		`[`: " ", // 左方括号
		`]`: " ", // 右方括号
		`{`: " ", // 左花括号
		`}`: " ", // 右花括号
	}

	// 引号类符号
	quotePatterns := map[string]string{
		`"`: " ", // 双引号
		`'`: " ", // 单引号
	}

	// 中文特殊字符
	chinesePatterns := map[string]string{
		`【`: " ", // 中文左方括号
		`】`: " ", // 中文右方括号
		`！`: " ", // 中文感叹号
		`？`: " ", // 中文问号
		`。`: " ", // 中文句号
		`，`: " ", // 中文逗号
		`、`: " ", // 中文顿号
		`；`: " ", // 中文分号
		`：`: " ", // 中文冒号
		`（`: " ", // 中文左括号
		`）`: " ", // 中文右括号
		`《`: " ", // 中文左书名号
		`》`: " ", // 中文右书名号
		`〈`: " ", // 中文左单书名号
		`〉`: " ", // 中文右单书名号
		`「`: " ", // 中文左直角引号
		`」`: " ", // 中文右直角引号
		`『`: " ", // 中文左双直角引号
		`』`: " ", // 中文右双直角引号
		`〔`: " ", // 中文左龟甲括号
		`〕`: " ", // 中文右龟甲括号
		`［`: " ", // 全角左方括号
		`］`: " ", // 全角右方括号
		`｛`: " ", // 全角左花括号
		`｝`: " ", // 全角右花括号
		`…`: " ", // 省略号
		`—`: " ", // 中文破折号
		`–`: " ", // 短破折号
	}

	// 中文引号类（单独处理避免与英文引号冲突）
	chineseQuotePatterns := map[string]string{
		"\u201C": " ", // 中文左双引号
		"\u201D": " ", // 中文右双引号
		"\u2018": " ", // 中文左单引号
		"\u2019": " ", // 中文右单引号
	}

	// 依次应用所有模式
	patternGroups := []map[string]string{
		basicPatterns,
		bracketPatterns,
		quotePatterns,
		chinesePatterns,
		chineseQuotePatterns,
	}

	for _, patterns := range patternGroups {
		for pattern, replacement := range patterns {
			result = strings.ReplaceAll(result, pattern, replacement)
		}
	}

	return result
}

// isOnlySpecialChars 检查字符串是否只包含特殊字符
func (s *StringSanitizer) isOnlySpecialChars(value string) bool {
	if value == "" {
		return false
	}

	// 移除所有特殊字符后，如果为空或只有空格，说明原字符串只包含特殊字符
	cleaned := s.specialCharsRegex.ReplaceAllString(value, "")
	return strings.TrimSpace(cleaned) == ""
}

// SanitizeForSheinTitle 为Shein标题清理字符串（更宽松的规则）
func (s *StringSanitizer) SanitizeForSheinTitle(title string) string {
	if title == "" {
		return title
	}

	// 对于标题，只移除最危险的字符，保留更多内容
	cleaned := strings.TrimSpace(title)

	// 只移除引号和一些控制字符
	dangerousChars := regexp.MustCompile(`['"<>|\\]`)
	cleaned = dangerousChars.ReplaceAllString(cleaned, "")

	// 清理多余的空格
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

	// 检查是否包含特殊字符
	if s.specialCharsRegex.MatchString(value) {
		return false
	}

	// 检查是否只包含空格
	if strings.TrimSpace(value) == "" {
		return false
	}

	// 检查长度（Shein可能有长度限制）
	if len(value) > 100 {
		return false
	}

	return true
}

// GetSanitizationSuggestion 获取清理建议
func (s *StringSanitizer) GetSanitizationSuggestion(original string) map[string]string {
	sanitized := s.SanitizeForSheinAttribute(original)

	return map[string]string{
		"original":  original,
		"sanitized": sanitized,
		"is_valid": func() string {
			if s.IsValidForSheinAttribute(sanitized) {
				return "true"
			}
			return "false"
		}(),
		"changes": func() string {
			if original == sanitized {
				return "无变化"
			}
			return "已清理特殊字符"
		}(),
	}
}

// removeRemainingSpecialChars 移除剩余的特殊字符
// 这是一个更严格的清理方法，只保留字母、数字和基本空格
func (s *StringSanitizer) removeRemainingSpecialChars(text string) string {
	return strings.Map(func(r rune) rune {
		// 只保留字母、数字、空格、小数点
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '.' {
			return r
		}
		// 允许一些基本的连接符，但转换为空格
		if r == '-' || r == '_' {
			return ' '
		}
		// 其他字符都移除
		return -1
	}, text)
}

// RemoveUnicodeControlChars 移除Unicode控制字符
func (s *StringSanitizer) RemoveUnicodeControlChars(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1 // 移除控制字符
		}
		return r
	}, text)
}

// 全局实例
var DefaultStringSanitizer = NewStringSanitizer()

// TruncateErrorMessage 截断并清理错误信息，确保适合数据库存储
// maxBytes: 最大字节数（考虑UTF-8编码）
// maxChars: 最大字符数
func (s *StringSanitizer) TruncateErrorMessage(errorMsg string, maxBytes int) string {
	if errorMsg == "" {
		return errorMsg
	}

	// 1. 移除控制字符和不可见字符
	cleaned := s.RemoveUnicodeControlChars(errorMsg)

	// 2. 替换可能导致编码问题的字符
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")   // 移除NULL字符
	cleaned = strings.ReplaceAll(cleaned, "\ufffd", "") // 移除替换字符

	// 3. 确保字符串是有效的UTF-8
	if !isValidUTF8(cleaned) {
		// 如果不是有效UTF-8，尝试修复
		cleaned = toValidUTF8(cleaned)
	}

	// 4. 按字节长度截断（考虑UTF-8编码）
	if len(cleaned) <= maxBytes {
		return cleaned
	}

	// 安全截断，避免截断UTF-8字符的中间
	truncated := truncateUTF8(cleaned, maxBytes-10) // 留10字节余量

	// 5. 添加截断标识
	if len(cleaned) > maxBytes {
		truncated += "...[截断]"
	}

	return truncated
}

// isValidUTF8 检查字符串是否为有效的UTF-8
func isValidUTF8(s string) bool {
	return strings.ToValidUTF8(s, "") == s
}

// toValidUTF8 将字符串转换为有效的UTF-8
func toValidUTF8(s string) string {
	return strings.ToValidUTF8(s, "?")
}

// truncateUTF8 安全地截断UTF-8字符串到指定字节数
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// 从maxBytes位置向前查找，找到完整的UTF-8字符边界
	for i := maxBytes; i > 0; i-- {
		if (s[i] & 0x80) == 0 { // ASCII字符
			return s[:i]
		}
		if (s[i] & 0xC0) == 0xC0 { // UTF-8字符的开始
			return s[:i]
		}
	}

	// 如果找不到合适的截断点，返回空字符串
	return ""
}

// 便捷函数
func SanitizeForSheinAttribute(value string) string {
	return DefaultStringSanitizer.SanitizeForSheinAttribute(value)
}

func SanitizeForSheinTitle(title string) string {
	return DefaultStringSanitizer.SanitizeForSheinTitle(title)
}

func IsValidForSheinAttribute(value string) bool {
	return DefaultStringSanitizer.IsValidForSheinAttribute(value)
}

// TruncateErrorMessage 截断错误信息的便捷函数
// 默认截断到400字节，为数据库字段留出余量
func TruncateErrorMessage(errorMsg string) string {
	return DefaultStringSanitizer.TruncateErrorMessage(errorMsg, 400)
}
