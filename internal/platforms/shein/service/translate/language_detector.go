// Package modules 提供SHEIN平台的语言检测功能
package translate

import (
	"strings"
)

// LanguageDetector 语言检测器
type LanguageDetector struct{}

// NewLanguageDetector 创建新的语言检测器
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{}
}

// DetectLanguage 检测文本的语言
func (d *LanguageDetector) DetectLanguage(title, description string) string {
	// 合并标题和描述进行检测
	text := title + " " + description
	text = strings.TrimSpace(text)

	if text == "" {
		return "en" // 默认返回英文
	}

	// 简单的语言检测：统计不同字符集的字符数量
	var japaneseCount, chineseCount, englishCount int

	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF): // 平假名和片假名
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF: // 中日韩统一表意文字
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	// 判断主要语言
	if japaneseCount > chineseCount && japaneseCount > englishCount {
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		return "zh" // 中文
	}

	return "en" // 默认英文
}

// IsJapanese 检查文本是否为日语
func (d *LanguageDetector) IsJapanese(text string) bool {
	return d.DetectLanguage(text, "") == "ja"
}

// IsChinese 检查文本是否为中文
func (d *LanguageDetector) IsChinese(text string) bool {
	return d.DetectLanguage(text, "") == "zh"
}

// IsEnglish 检查文本是否为英语
func (d *LanguageDetector) IsEnglish(text string) bool {
	return d.DetectLanguage(text, "") == "en"
}

// GetCharacterCounts 获取不同字符集的字符数量统计
func (d *LanguageDetector) GetCharacterCounts(text string) (japanese, chinese, english int) {
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF): // 平假名和片假名
			japanese++
		case r >= 0x4E00 && r <= 0x9FFF: // 中日韩统一表意文字
			chinese++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			english++
		}
	}
	return japanese, chinese, english
}

// GetTargetLanguagesByRegion 根据区域获取目标语言列表
func GetTargetLanguagesByRegion(region string) []string {
	// 根据不同的区域返回不同的目标语言
	switch region {
	case "US", "MX":
		// 美国和墨西哥站点需要翻译为西班牙语
		return []string{"en", "es"}
	case "FR", "DE", "IT", "ES":
		// 欧洲站点需要翻译为德语、西班牙语、法语、意大利语
		return []string{"de", "es", "fr", "it", "en"}
	case "JP":
		// 日本站点需要日语
		return []string{"ja", "en"}
	case "SA", "AE":
		// 沙特和阿联酋站点需要阿拉伯语
		return []string{"ar", "en"}
	default:
		// 默认返回空列表，表示不支持的区域
		return []string{"en"}
	}
}
