// Package modules 提供SHEIN平台SKC翻译处理功能
package modules

import (
	"strings"
	"task-processor/internal/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKCTranslationHandler SKC翻译处理器
type SKCTranslationHandler struct {
	taskContext *TaskContext
}

// NewSKCTranslationHandler 创建新的SKC翻译处理器
func NewSKCTranslationHandler(taskContext *TaskContext) *SKCTranslationHandler {
	return &SKCTranslationHandler{
		taskContext: taskContext,
	}
}

// CreateSKC 创建SKC的工厂函数
func (h *SKCTranslationHandler) CreateSKC(ctx *TaskContext, params SKCCreationParams) product.SKC {
	// 1. 获取目标语言列表
	targetLanguages := GetTargetLanguagesByRegion(ctx.Task.Region)

	// 2. 查找标题作为翻译源
	sourceTitle := h.findBestSourceTitle(ctx, params)

	// 3. 检测源标题的语言
	sourceLang := h.detectTitleLanguage(sourceTitle)

	// 4. 初始化多语言内容结构
	multiLanguageNameList := h.initializeMultiLanguageContent(targetLanguages)

	// 5. 翻译到所有目标语言
	h.translateToAllLanguages(ctx, sourceTitle, sourceLang, &multiLanguageNameList)

	// 6. 选择主要显示语言
	primaryLanguageContent := h.selectPrimaryDisplayLanguage(targetLanguages, multiLanguageNameList, sourceTitle)

	skc := product.SKC{
		SaleAttribute: product.SaleAttribute{
			AttributeID:        params.AttributeID,
			AttributeValueID:   params.AttributeValueID,
			IsSPPSaleAttribute: false,
			PreFillSpec:        false,
		},
		SKUS:                    params.SKUS,
		ImageInfo:               params.ImageInfo,
		SiteDetailImageInfoList: []product.SiteDetailImageInfo{},
		ShelfWay:                1,
		ShelfRequire:            0,
		MultiLanguageName:       primaryLanguageContent,
		MultiLanguageNameList:   multiLanguageNameList,
		Sort:                    params.Sort,
	}
	return skc
}

// findBestSourceTitle 查找最佳的源标题作为翻译源
func (h *SKCTranslationHandler) findBestSourceTitle(ctx *TaskContext, params SKCCreationParams) string {
	logrus.Debugf("🔍 开始查找源标题...")

	// 优先尝试根据SKU的SupplierSKU反向查找对应的ASIN，然后匹配变体标题
	if ctx.Variants != nil && len(*ctx.Variants) > 0 && len(params.SKUS) > 0 {
		// 获取当前SKC对应的SupplierSKU（从第一个SKU获取）
		if len(params.SKUS) > 0 && params.SKUS[0].SupplierSKU != "" {
			supplierSKU := params.SKUS[0].SupplierSKU
			logrus.Debugf("🎯 通过SupplierSKU %s 反向查找对应的ASIN", supplierSKU)

			// 通过AsinSkuMap反向查找ASIN
			var targetASIN string
			if ctx.AsinSkuMap != nil {
				for asin, sku := range ctx.AsinSkuMap {
					if sku == supplierSKU {
						targetASIN = asin
						logrus.Debugf("✅ 找到对应的ASIN: %s -> %s", supplierSKU, targetASIN)
						break
					}
				}
			}

			// 如果找到了ASIN，查找对应的变体标题
			if targetASIN != "" {
				for _, variant := range *ctx.Variants {
					if variant.Asin == targetASIN && variant.Title != "" {
						logrus.Infof("✅ 找到匹配变体标题: ASIN=%s, Title=%s", variant.Asin, variant.Title)
						return variant.Title
					}
				}
				logrus.Debugf("⚠️ ASIN %s 对应的变体标题为空", targetASIN)
			} else {
				logrus.Debugf("⚠️ 未找到SupplierSKU %s 对应的ASIN", supplierSKU)
			}
		}

		// 如果没有找到匹配的变体标题，尝试使用任何有效的变体标题
		for _, variant := range *ctx.Variants {
			if variant.Title != "" {
				logrus.Infof("✅ 使用其他变体标题: ASIN=%s, Title=%s", variant.Asin, variant.Title)
				return variant.Title
			}
		}
	}

	// 如果没有找到变体标题，尝试使用产品标题
	if ctx.AmazonProduct.Title != "" {
		logrus.Infof("✅ 使用产品标题: %s", ctx.AmazonProduct.Title)
		return ctx.AmazonProduct.Title
	}

	logrus.Warnf("⚠️ 未找到有效的标题")
	return ""
}

// detectTitleLanguage 检测标题的语言
func (h *SKCTranslationHandler) detectTitleLanguage(title string) string {
	title = strings.TrimSpace(title)

	if title == "" {
		return "en" // 默认返回英文
	}

	// 简单的语言检测：统计不同字符集的字符数量
	var japaneseCount, chineseCount, englishCount int

	for _, r := range title {
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
		logrus.Infof("🔍 检测到标题语言: 日语")
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		logrus.Infof("🔍 检测到标题语言: 中文")
		return "zh"
	}

	logrus.Infof("🔍 检测到标题语言: 英文")
	return "en"
}

// initializeMultiLanguageContent 初始化多语言内容结构
func (h *SKCTranslationHandler) initializeMultiLanguageContent(targetLanguages []string) []product.LanguageContent {
	logrus.Debugf("🌐 初始化多语言内容结构，目标语言数量: %d", len(targetLanguages))

	multiLanguageNameList := make([]product.LanguageContent, 0, len(targetLanguages))

	for _, lang := range targetLanguages {
		multiLanguageNameList = append(multiLanguageNameList, product.LanguageContent{
			Language: lang,
			Name:     "", // 初始化为空，后续通过翻译填充
		})
		logrus.Debugf("📝 初始化语言: %s", lang)
	}

	return multiLanguageNameList
}

// translateToAllLanguages 翻译到所有目标语言
func (h *SKCTranslationHandler) translateToAllLanguages(ctx *TaskContext, sourceTitle string, sourceLang string, multiLanguageNameList *[]product.LanguageContent) {
	if ctx.ShopClient == nil || sourceTitle == "" {
		logrus.Warnf("⚠️ 跳过翻译：ShopClient为空(%v) 或 源标题为空(%v)", ctx.ShopClient == nil, sourceTitle == "")
		return
	}

	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]

		// 如果目标语言与源语言相同，直接设置原标题
		if langContent.Language == sourceLang {
			langContent.Name = sourceTitle
			logrus.Debugf("✅ 设置源语言(%s)标题: %s", sourceLang, sourceTitle)
			continue
		}

		// 翻译到目标语言
		translatedTitle, err := ctx.ShopClient.Translate(sourceTitle, sourceLang, langContent.Language)
		if err != nil {
			logrus.Warnf("❌ 翻译到语言 %s 失败: %v，使用源标题作为后备", langContent.Language, err)
			langContent.Name = sourceTitle // 翻译失败时使用源标题作为后备
			continue
		}

		langContent.Name = translatedTitle
	}
}

// selectPrimaryDisplayLanguage 选择主要显示语言
func (h *SKCTranslationHandler) selectPrimaryDisplayLanguage(targetLanguages []string, multiLanguageNameList []product.LanguageContent, sourceTitle string) product.LanguageContent {
	if len(targetLanguages) == 0 {
		// 如果没有目标语言，尝试从多语言列表中选择第一个有效的
		if len(multiLanguageNameList) > 0 && multiLanguageNameList[0].Name != "" {
			logrus.Infof("📋 无目标语言，使用多语言列表第一项作为主要显示语言: %s", multiLanguageNameList[0].Language)
			return multiLanguageNameList[0]
		}
		// 最后的后备方案
		logrus.Infof("📋 无目标语言，使用源标题作为主要显示语言")
		return product.LanguageContent{
			Language: "en",
			Name:     sourceTitle,
		}
	}

	// 使用第一个目标语言作为主要显示语言
	primaryTargetLang := targetLanguages[0]
	logrus.Infof("🎯 选择主要显示语言: %s", primaryTargetLang)

	// 在多语言列表中查找对应的翻译内容
	for _, langContent := range multiLanguageNameList {
		if langContent.Language == primaryTargetLang && langContent.Name != "" {
			logrus.Infof("✅ 使用目标语言 %s 作为主要显示标题: %s", primaryTargetLang, langContent.Name)
			return langContent
		}
	}

	// 如果没有找到目标语言的翻译，使用源标题作为后备
	logrus.Warnf("⚠️ 未找到语言 %s 的翻译内容，使用源标题作为后备", primaryTargetLang)
	return product.LanguageContent{
		Language: primaryTargetLang,
		Name:     sourceTitle,
	}
}
