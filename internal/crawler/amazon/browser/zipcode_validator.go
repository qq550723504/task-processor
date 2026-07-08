// Package browser 提供Amazon浏览器自动化的邮编验证功能
package browser

import (
	"fmt"
	"net/url"
	"strings"
	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
)

// ZipcodeValidator 邮编验证器
type ZipcodeValidator struct {
	getter    *ZipcodeGetter
	targetURL string
}

type targetContextState struct {
	CurrentText      string
	ExtractedZipcode string
	InferredCountry  string
	TargetCountry    string
	Matched          bool
}

// NewZipcodeValidator 创建邮编验证器实例
func NewZipcodeValidator() *ZipcodeValidator {
	return &ZipcodeValidator{
		getter: NewZipcodeGetter(),
	}
}

func (zv *ZipcodeValidator) SetTargetURL(targetURL string) {
	if zv == nil {
		return
	}
	zv.targetURL = targetURL
}

// MatchesTargetContext 判断当前页面配送上下文是否已经落在目标国家。
// 对美国站这类场景，如果当前文本里已经是合法美国 ZIP，也认为无需再设置默认邮编。
func (zv *ZipcodeValidator) MatchesTargetContext(page playwright.Page) (bool, error) {
	state, err := zv.inspectTargetContext(page)
	if err != nil {
		return false, err
	}

	logger.GetGlobalLogger("crawler/amazon").Infof(
		"配送上下文判断: current_text=%q extracted_zipcode=%q inferred_country=%q target_country=%q matched=%t",
		state.CurrentText,
		state.ExtractedZipcode,
		state.InferredCountry,
		state.TargetCountry,
		state.Matched,
	)

	return state.Matched, nil
}

func (zv *ZipcodeValidator) inspectTargetContext(page playwright.Page) (*targetContextState, error) {
	if zv == nil || zv.getter == nil {
		return nil, fmt.Errorf("zipcode validator is not initialized")
	}

	currentText, err := zv.getter.GetCurrentZipcode(page)
	if err != nil {
		return nil, fmt.Errorf("获取当前邮编失败: %w", err)
	}

	targetCountry := inferCountryFromTargetURLForValidation(zv.targetURL)
	if targetCountry == "" {
		return &targetContextState{CurrentText: currentText}, nil
	}

	extractedZipcode := ExtractZipcode(currentText)
	inferenceSource := strings.TrimSpace(currentText)
	if extractedZipcode != "" {
		inferenceSource = extractedZipcode
	}

	return &targetContextState{
		CurrentText:      currentText,
		ExtractedZipcode: extractedZipcode,
		InferredCountry:  inferCountryFromZipcodeForValidation(inferenceSource),
		TargetCountry:    targetCountry,
		Matched:          textMatchesTargetContext(currentText, targetCountry),
	}, nil
}

// VerifyZipcode 验证邮编是否设置成功
func (zv *ZipcodeValidator) VerifyZipcode(page playwright.Page, expectedZipcode string) (bool, error) {
	// 获取当前邮编并验证
	currentZipcode, err := zv.getter.GetCurrentZipcode(page)
	if err != nil {
		return false, fmt.Errorf("获取当前邮编失败: %w", err)
	}

	// 清理文本：移除所有空白字符（包括换行、制表符等）和特殊字符
	cleanCurrent := cleanZipcodeText(currentZipcode)
	cleanExpected := cleanZipcodeText(expectedZipcode)

	logger.GetGlobalLogger("crawler/amazon").Infof("验证邮编 - 期望: '%s' (清理后: '%s'), 当前: '%s' (清理后: '%s')",
		expectedZipcode, cleanExpected, currentZipcode, cleanCurrent)

	// 1. 完全匹配（清理后）
	if cleanCurrent == cleanExpected {
		logger.GetGlobalLogger("crawler/amazon").Infof("邮编完全匹配")
		return true, nil
	}

	// 2. 提取邮编的主要部分（outward code）进行匹配
	// 适用于英国等站点，页面可能只显示部分邮编
	// 例如: 期望 "SW1A1AA"，页面显示 "LondonSW1A1"
	expectedCore := extractZipcodeCore(cleanExpected)
	if expectedCore != "" && strings.Contains(cleanCurrent, expectedCore) {
		logger.GetGlobalLogger("crawler/amazon").Infof("邮编核心部分匹配: '%s' 包含在 '%s' 中", expectedCore, cleanCurrent)
		return true, nil
	}

	// 3. 加拿大邮编：页面可能显示实际配送地址的 FSA（前3位），与设置的邮编不同
	// 只要当前显示的是合法的加拿大 FSA 格式，就认为邮编设置成功
	if isCanadianZipcode(cleanExpected) && isValidCanadianFSA(cleanCurrent) {
		logger.GetGlobalLogger("crawler/amazon").Infof("加拿大邮编 FSA 验证通过: 期望 '%s'，页面显示 '%s'（配送地址 FSA）", cleanExpected, cleanCurrent)
		return true, nil
	}

	// 4. 对于某些站点(如沙特),页面显示的是城市名称而非邮编
	expectedCity := mapZipcodeToCity(expectedZipcode)
	if expectedCity != "" && strings.Contains(cleanCurrent, strings.ReplaceAll(expectedCity, " ", "")) {
		logger.GetGlobalLogger("crawler/amazon").Infof("邮编映射到城市名称匹配: %s", expectedCity)
		return true, nil
	}

	// 5. 如果当前位置文本已经明确显示目标国家/目标配送上下文，则无需再次设置邮编。
	targetCountry := inferTargetCountry(zv.targetURL, expectedZipcode)
	if targetCountry != "" && locationMatchesTargetCountry(currentZipcode, targetCountry) {
		logger.GetGlobalLogger("crawler/amazon").Infof("当前位置已命中目标国家/配送上下文，跳过邮编设置: target=%s current=%s", targetCountry, currentZipcode)
		return true, nil
	}

	logger.GetGlobalLogger("crawler/amazon").Warnf("邮编验证失败 - 期望: '%s', 当前: '%s'", cleanExpected, cleanCurrent)
	return false, nil
}

func inferTargetCountry(targetURL, expectedZipcode string) string {
	if country := inferCountryFromTargetURLForValidation(targetURL); country != "" {
		return country
	}
	return inferCountryFromZipcodeForValidation(expectedZipcode)
}

func inferCountryFromTargetURLForValidation(targetURL string) string {
	if targetURL == "" {
		return ""
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	host = strings.TrimPrefix(host, "www.")

	targetCountries := map[string]string{
		"amazon.com":    "United States",
		"amazon.ca":     "Canada",
		"amazon.co.uk":  "United Kingdom",
		"amazon.de":     "Germany",
		"amazon.fr":     "France",
		"amazon.it":     "Italy",
		"amazon.es":     "Spain",
		"amazon.co.jp":  "Japan",
		"amazon.com.au": "Australia",
		"amazon.in":     "India",
		"amazon.com.mx": "Mexico",
		"amazon.com.br": "Brazil",
		"amazon.nl":     "Netherlands",
		"amazon.se":     "Sweden",
		"amazon.pl":     "Poland",
	}

	return targetCountries[host]
}

func inferCountryFromZipcodeForValidation(zipcode string) string {
	return inferCountryFromZipcodeValue(zipcode)
}

func locationMatchesTargetCountry(currentText, targetCountry string) bool {
	raw := strings.TrimSpace(currentText)
	if raw == "" {
		return false
	}

	normalizedRaw := normalizeLocationText(raw)
	for _, keyword := range countryContextKeywords(targetCountry) {
		if strings.Contains(normalizedRaw, normalizeLocationText(keyword)) {
			return true
		}
	}

	return false
}

func textMatchesTargetContext(currentText, targetCountry string) bool {
	raw := strings.TrimSpace(currentText)
	if raw == "" {
		return false
	}

	candidate := raw
	if extractedZipcode := ExtractZipcode(raw); extractedZipcode != "" {
		candidate = extractedZipcode
	}

	if inferredCountry := inferCountryFromZipcodeForValidation(candidate); inferredCountry != "" &&
		strings.EqualFold(inferredCountry, targetCountry) {
		return true
	}

	return locationMatchesTargetCountry(raw, targetCountry)
}

func normalizeLocationText(text string) string {
	text = strings.ToUpper(strings.TrimSpace(text))
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\t", "")
	text = strings.ReplaceAll(text, "-", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, ".", "")
	return text
}

func countryContextKeywords(targetCountry string) []string {
	switch strings.ToLower(strings.TrimSpace(targetCountry)) {
	case "japan":
		return []string{"Japan", "日本", "Tokyo", "東京都", "Osaka", "大阪", "Kyoto", "京都", "Kanagawa", "神奈川", "Saitama", "埼玉", "Chiba", "千葉", "Hokkaido", "北海道", "Fukuoka", "福岡"}
	case "united kingdom":
		return []string{"United Kingdom", "Great Britain", "England", "London"}
	case "canada":
		return []string{"Canada", "Toronto", "Ontario", "Vancouver"}
	case "united states":
		return []string{"United States", "USA"}
	default:
		return []string{targetCountry}
	}
}

// mapZipcodeToCity 将邮编映射到城市名称(用于验证)
func mapZipcodeToCity(zipcode string) string {
	// 沙特城市映射
	saudiCityMap := map[string]string{
		"11564": "Riyadh",   // 利雅得
		"21432": "Jeddah",   // 吉达
		"23218": "Dammam",   // 达曼
		"31952": "Mecca",    // 麦加
		"24231": "Medina",   // 麦地那
		"32272": "Khobar",   // 胡拜尔
		"13521": "Buraidah", // 布赖代
		"51431": "Abha",     // 艾卜哈
		"82723": "Tabuk",    // 塔布克
		"41311": "Hail",     // 哈伊勒
	}

	// 阿联酋城市映射
	uaeCityMap := map[string]string{
		"00000": "Dubai",     // 迪拜
		"00001": "Abu Dhabi", // 阿布扎比
		"00002": "Sharjah",   // 沙迦
		"00003": "Ajman",     // 阿治曼
	}

	// 先尝试沙特映射
	if city, exists := saudiCityMap[zipcode]; exists {
		return city
	}

	// 再尝试阿联酋映射
	if city, exists := uaeCityMap[zipcode]; exists {
		return city
	}

	return ""
}

// extractZipcodeCore 提取邮编的核心部分用于匹配
// 对于不同格式的邮编，提取最有代表性的部分
func extractZipcodeCore(cleanedZipcode string) string {
	if cleanedZipcode == "" {
		return ""
	}

	// 英国邮编格式: SW1A1AA -> 提取 SW1A1 (去掉最后2个字符，通常是inward code)
	// 美国邮编格式: 10001 -> 保持原样
	// 加拿大邮编格式: M5H2N2 -> 提取 M5H (前3个字符)

	length := len(cleanedZipcode)

	// 如果邮编长度 >= 6，可能是英国或加拿大格式
	if length >= 6 {
		// 英国邮编通常是 5-7 个字符（去掉空格后）
		// 提取前面的主要部分（去掉最后 1-3 个字符）
		// 例如: SW1A1AA -> SW1A1, SW1A -> SW1A
		if length == 7 || length == 6 {
			return cleanedZipcode[:length-2] // 去掉最后2个字符
		}
		if length == 5 {
			return cleanedZipcode[:length-1] // 去掉最后1个字符
		}
	}

	// 对于较短的邮编（如美国5位数字），保持原样
	if length == 5 {
		return cleanedZipcode
	}

	// 默认返回前面大部分内容（至少保留3个字符）
	if length > 3 {
		return cleanedZipcode[:length-1]
	}

	return cleanedZipcode
}

// cleanZipcodeText 清理邮编文本，移除所有空白字符和特殊字符
func cleanZipcodeText(text string) string {
	// 移除所有空白字符（空格、换行、制表符等）
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\t", "")

	// 移除零宽字符和其他不可见字符
	text = strings.Map(func(r rune) rune {
		// 保留字母和数字
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		// 移除其他字符
		return -1
	}, text)

	return strings.ToUpper(text)
}

// isCanadianZipcode 判断清理后的邮编是否为加拿大格式（6位 A1A1A1）
func isCanadianZipcode(cleaned string) bool {
	if len(cleaned) != 6 {
		return false
	}
	// 加拿大邮编格式: 字母-数字-字母-数字-字母-数字
	for i, r := range cleaned {
		isLetter := (r >= 'A' && r <= 'Z')
		isDigit := (r >= '0' && r <= '9')
		if i%2 == 0 && !isLetter {
			return false
		}
		if i%2 == 1 && !isDigit {
			return false
		}
	}
	return true
}

// isValidCanadianFSA 判断文本是否为合法的加拿大 FSA（前向码，3位：字母-数字-字母）
func isValidCanadianFSA(text string) bool {
	if len(text) != 3 {
		return false
	}
	r0, r1, r2 := rune(text[0]), rune(text[1]), rune(text[2])
	return (r0 >= 'A' && r0 <= 'Z') && (r1 >= '0' && r1 <= '9') && (r2 >= 'A' && r2 <= 'Z')
}
