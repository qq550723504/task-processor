package filter

import (
	"regexp"
	"strings"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// DetectorUtils 检测器工具函数
type DetectorUtils struct {
	logger *logrus.Entry
}

// NewDetectorUtils 创建检测器工具
func NewDetectorUtils(logger *logrus.Entry) *DetectorUtils {
	return &DetectorUtils{
		logger: logger,
	}
}

// CheckKeywords 检查关键词
func (u *DetectorUtils) CheckKeywords(texts []string, keywords []string, category string, result *ProhibitedItemResult) {
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, keyword := range keywords {
			if strings.Contains(lowerText, strings.ToLower(keyword)) {
				result.ViolatedItems = append(result.ViolatedItems, keyword)
				if result.ViolatedCategory == "" {
					result.ViolatedCategory = category
				}
			}
		}
	}
}

// CheckPatterns 检查正则模式
func (u *DetectorUtils) CheckPatterns(texts []string, patterns []*regexp.Regexp, category string, result *ProhibitedItemResult) {
	for _, text := range texts {
		for _, pattern := range patterns {
			if pattern.MatchString(text) {
				result.ViolatedItems = append(result.ViolatedItems, pattern.String())
				if result.ViolatedCategory == "" {
					result.ViolatedCategory = category
				}
			}
		}
	}
}

// CalculateConfidence 计算置信度
func (u *DetectorUtils) CalculateConfidence(violatedItems []string) float64 {
	if len(violatedItems) == 0 {
		return 0.0
	}

	// 高置信度关键词（明确的违禁品）
	highConfidenceKeywords := []string{
		"gun", "rifle", "pistol", "ammunition", "bullet", "explosive", "bomb",
		"cocaine", "heroin", "marijuana", "porn", "sex toy", "live animal",
		"枪", "步枪", "手枪", "弹药", "子弹", "爆炸", "炸弹", "毒品", "色情",
	}

	// 中等置信度关键词（需要上下文验证）
	mediumConfidenceKeywords := []string{
		"weapon", "tactical", "replica", "fake", "chemical", "prescription",
		"武器", "战术", "仿制", "假货", "化学品", "处方药",
	}

	// 低置信度关键词（容易误判）
	lowConfidenceKeywords := []string{
		"adult", "copy", "medicine", "pet", "成人", "复制", "药品", "宠物",
	}

	highCount := 0
	mediumCount := 0
	lowCount := 0

	for _, item := range violatedItems {
		lowerItem := strings.ToLower(item)

		// 检查是否为高置信度关键词
		isHigh := false
		for _, keyword := range highConfidenceKeywords {
			if strings.Contains(lowerItem, strings.ToLower(keyword)) {
				highCount++
				isHigh = true
				break
			}
		}

		if isHigh {
			continue
		}

		// 检查是否为中等置信度关键词
		isMedium := false
		for _, keyword := range mediumConfidenceKeywords {
			if strings.Contains(lowerItem, strings.ToLower(keyword)) {
				mediumCount++
				isMedium = true
				break
			}
		}

		if isMedium {
			continue
		}

		// 检查是否为低置信度关键词
		for _, keyword := range lowConfidenceKeywords {
			if strings.Contains(lowerItem, strings.ToLower(keyword)) {
				lowCount++
				break
			}
		}
	}

	// 计算加权置信度
	confidence := float64(highCount)*0.9 + float64(mediumCount)*0.6 + float64(lowCount)*0.3

	// 如果只有低置信度关键词，降低整体置信度
	if highCount == 0 && mediumCount == 0 && lowCount > 0 {
		confidence = confidence * 0.5
	}

	// 限制在0.0-1.0范围内
	if confidence > 1.0 {
		confidence = 1.0
	}

	u.logger.Debugf("置信度计算: 高=%d, 中=%d, 低=%d, 最终置信度=%.2f",
		highCount, mediumCount, lowCount, confidence)

	return confidence
}

// ExtractProductTexts 提取产品文本信息（仅检测标题）
func (u *DetectorUtils) ExtractProductTexts(amazonProduct *model.Product) []string {
	texts := []string{}

	if amazonProduct != nil && strings.TrimSpace(amazonProduct.Title) != "" {
		texts = append(texts, amazonProduct.Title)
		u.logger.Debugf("🔍 提取产品标题用于违禁词检测: %s", amazonProduct.Title)
	} else {
		u.logger.Warn("⚠️ 产品标题为空，跳过违禁词检测")
	}

	return texts
}

// ExtractProductCategories 提取产品分类信息
func (u *DetectorUtils) ExtractProductCategories(amazonProduct *model.Product) []string {
	categories := []string{}

	if amazonProduct != nil && len(amazonProduct.Categories) > 0 {
		categories = amazonProduct.Categories
		u.logger.Debugf("🔍 提取产品分类用于违禁词检测: %v", categories)
	} else {
		u.logger.Debug("⚠️ 产品分类为空")
	}

	// 添加其他分类相关字段
	if amazonProduct.BsCategory != "" {
		categories = append(categories, amazonProduct.BsCategory)
	}
	if amazonProduct.RootBsCategory != "" {
		categories = append(categories, amazonProduct.RootBsCategory)
	}
	if amazonProduct.Department != "" {
		categories = append(categories, amazonProduct.Department)
	}

	return categories
}

// IsLegitimateProductCategory 检查是否为合法产品类别
func (u *DetectorUtils) IsLegitimateProductCategory(categories []string, productTexts []string) bool {
	// 明确的合法产品类别
	legitimateCategories := []string{
		"pet supplies", "pet accessories", "pet food", "dog supplies", "cat supplies",
		"clothing", "shoes", "accessories", "jewelry", "watches",
		"home & kitchen", "home decor", "furniture", "bedding", "bath",
		"electronics", "computers", "phones", "tablets", "cameras",
		"books", "movies", "music", "games", "toys",
		"sports", "fitness", "outdoor", "camping", "hiking",
		"automotive", "tools", "hardware", "garden", "patio",
		"beauty", "personal care", "health", "wellness",
		"office supplies", "school supplies", "art supplies",
		"baby", "kids", "maternity",
		"grocery", "food", "beverages", "snacks",
		"宠物用品", "服装", "鞋子", "家居", "电子产品", "图书", "运动", "美容", "食品",
	}

	// 检查产品分类
	for _, category := range categories {
		lowerCategory := strings.ToLower(category)
		for _, legitCategory := range legitimateCategories {
			if strings.Contains(lowerCategory, strings.ToLower(legitCategory)) {
				u.logger.Debugf("✅ 发现合法产品类别: %s", category)
				return true
			}
		}
	}

	// 检查产品标题中的合法关键词
	for _, text := range productTexts {
		lowerText := strings.ToLower(text)
		for _, legitCategory := range legitimateCategories {
			if strings.Contains(lowerText, strings.ToLower(legitCategory)) {
				u.logger.Debugf("✅ 产品标题包含合法关键词: %s", legitCategory)
				return true
			}
		}
	}

	return false
}
