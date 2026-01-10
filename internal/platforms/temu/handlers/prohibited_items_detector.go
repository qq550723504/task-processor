package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"task-processor/internal/domain/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ProhibitedItemsDetector TEMU违禁品检测器
type ProhibitedItemsDetector struct {
	logger           *logrus.Entry
	staticKeywords   map[string][]string
	dynamicPatterns  map[string][]*regexp.Regexp
	configPath       string
	categoryKeywords map[string][]string
}

// ProhibitedItemsConfig 违禁品配置结构
type ProhibitedItemsConfig struct {
	StaticKeywords   map[string][]string `json:"static_keywords"`
	DynamicPatterns  map[string][]string `json:"dynamic_patterns"`
	CategoryKeywords map[string][]string `json:"category_keywords"`
	LastUpdated      string              `json:"last_updated"`
	Version          string              `json:"version"`
	Platform         string              `json:"platform"`
}

// ProhibitedItemResult 违禁品检测结果
type ProhibitedItemResult struct {
	IsProhibited     bool     `json:"is_prohibited"`
	ViolatedItems    []string `json:"violated_items"`
	ViolatedCategory string   `json:"violated_category"`
	Confidence       float64  `json:"confidence"`
	Reason           string   `json:"reason"`
}

// NewProhibitedItemsDetector 创建违禁品检测器
func NewProhibitedItemsDetector() *ProhibitedItemsDetector {
	detector := &ProhibitedItemsDetector{
		logger:           logrus.WithField("handler", "ProhibitedItemsDetector"),
		staticKeywords:   make(map[string][]string),
		dynamicPatterns:  make(map[string][]*regexp.Regexp),
		categoryKeywords: make(map[string][]string),
		configPath:       "data/prohibited_items_temu.json",
	}

	if err := detector.loadConfig(); err != nil {
		detector.logger.WithError(err).Warn("加载违禁品配置失败，使用默认配置")
		detector.loadDefaultConfig()
	}

	return detector
}

// Name 返回处理器名称
func (d *ProhibitedItemsDetector) Name() string {
	return "TEMU违禁品检测器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (d *ProhibitedItemsDetector) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	d.logger.Info("🔍 开始检测违禁品")

	// 检查Amazon产品数据
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		return fmt.Errorf("Amazon产品数据为空")
	}

	// 执行违禁品检测
	result := d.DetectProhibitedItems(amazonProduct)

	if result.IsProhibited {
		d.logger.Errorf("❌ 检测到违禁品: %s", result.Reason)
		d.logger.Errorf("   违禁类别: %s", result.ViolatedCategory)
		d.logger.Errorf("   违禁关键词: %v", result.ViolatedItems)
		d.logger.Errorf("   置信度: %.2f", result.Confidence)

		// 打印详细的产品信息用于分析
		d.logger.Errorf("📋 产品详细信息:")
		d.logger.Errorf("   Amazon标题: %s", amazonProduct.Title)
		d.logger.Errorf("   Amazon品牌: %s", amazonProduct.Brand)
		d.logger.Errorf("   Amazon分类: %v", amazonProduct.Categories)
		d.logger.Errorf("   Amazon部门: %s", amazonProduct.Department)
		d.logger.Errorf("   Amazon制造商: %s", amazonProduct.Manufacturer)

		// 打印产品特性（可能包含违禁词）
		if len(amazonProduct.Features) > 0 {
			d.logger.Errorf("   Amazon特性:")
			for i, feature := range amazonProduct.Features {
				if i < 5 { // 只显示前5个特性，避免日志过长
					d.logger.Errorf("     - %s", feature)
				}
			}
		}

		// 打印产品详情（可能包含违禁词）
		if len(amazonProduct.ProductDetails) > 0 {
			d.logger.Errorf("   Amazon产品详情:")
			for i, detail := range amazonProduct.ProductDetails {
				if i < 5 { // 只显示前5个详情，避免日志过长
					d.logger.Errorf("     %s: %s", detail.Type, detail.Value)
				}
			}
		}

		// 返回不可重试错误
		return types.NewNonRetryableError(
			fmt.Sprintf("产品包含违禁品内容: %s (类别: %s)", result.Reason, result.ViolatedCategory),
			fmt.Errorf("违禁品检测失败: %v", result.ViolatedItems),
		)
	}

	d.logger.Info("✅ 违禁品检测通过")
	return nil
}

// Handle 兼容原有的Handler接口
func (d *ProhibitedItemsDetector) Handle(ctx pipeline.TaskContext) error {
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return d.HandleTemu(temuCtx)
	}
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// DetectProhibitedItems 检测违禁品（优化版）
func (d *ProhibitedItemsDetector) DetectProhibitedItems(amazonProduct *model.Product) *ProhibitedItemResult {
	result := &ProhibitedItemResult{
		IsProhibited:  false,
		ViolatedItems: []string{},
		Confidence:    0.0,
	}

	// 提取产品文本信息（标题）
	productTexts := d.extractProductTexts(amazonProduct)
	if len(productTexts) == 0 {
		d.logger.Warn("⚠️ 没有可检测的产品文本，跳过违禁词检测")
		return result
	}

	// 提取产品分类信息
	categories := d.extractProductCategories(amazonProduct)

	// 首先检查是否为明确的合法产品类别
	if d.isLegitimateProductCategory(categories, productTexts) {
		d.logger.Info("✅ 检测到合法产品类别，跳过违禁品检测")
		return result
	}

	// 使用改进的检测逻辑：标题和分类都需要包含违禁词才确定为违禁品
	d.detectProhibitedItemsWithContext(productTexts, categories, result)

	// 计算总体置信度
	if len(result.ViolatedItems) > 0 {
		result.Confidence = d.calculateConfidence(result.ViolatedItems)

		// 如果置信度过低，不标记为违禁品
		if result.Confidence < 0.5 {
			d.logger.Infof("⚠️ 置信度过低(%.2f)，不标记为违禁品: %v", result.Confidence, result.ViolatedItems)
			result.IsProhibited = false
			result.ViolatedItems = []string{}
			result.ViolatedCategory = ""
			result.Confidence = 0.0
			return result
		}

		result.IsProhibited = true
		result.Reason = fmt.Sprintf("检测到%d个违禁关键词", len(result.ViolatedItems))
	}

	return result
}

// isLegitimateProductCategory 检查是否为合法产品类别
func (d *ProhibitedItemsDetector) isLegitimateProductCategory(categories []string, productTexts []string) bool {
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
				d.logger.Debugf("✅ 发现合法产品类别: %s", category)
				return true
			}
		}
	}

	// 检查产品标题中的合法关键词
	for _, text := range productTexts {
		lowerText := strings.ToLower(text)
		for _, legitCategory := range legitimateCategories {
			if strings.Contains(lowerText, strings.ToLower(legitCategory)) {
				d.logger.Debugf("✅ 产品标题包含合法关键词: %s", legitCategory)
				return true
			}
		}
	}

	return false
}

// extractProductTexts 提取产品文本信息（仅检测标题）
func (d *ProhibitedItemsDetector) extractProductTexts(amazonProduct *model.Product) []string {
	texts := []string{}

	// 直接从强类型结构体获取标题
	if amazonProduct != nil && strings.TrimSpace(amazonProduct.Title) != "" {
		texts = append(texts, amazonProduct.Title)
		d.logger.Debugf("🔍 提取产品标题用于违禁词检测: %s", amazonProduct.Title)
	} else {
		d.logger.Warn("⚠️ 产品标题为空，跳过违禁词检测")
	}

	return texts
}

// extractProductCategories 提取产品分类信息
func (d *ProhibitedItemsDetector) extractProductCategories(amazonProduct *model.Product) []string {
	categories := []string{}

	if amazonProduct != nil && len(amazonProduct.Categories) > 0 {
		categories = amazonProduct.Categories
		d.logger.Debugf("🔍 提取产品分类用于违禁词检测: %v", categories)
	} else {
		d.logger.Debug("⚠️ 产品分类为空")
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

// detectProhibitedItemsWithContext 使用上下文信息检测违禁品
func (d *ProhibitedItemsDetector) detectProhibitedItemsWithContext(productTexts []string, categories []string, result *ProhibitedItemResult) {
	// 定义需要上下文验证的敏感词汇
	contextSensitiveWords := map[string][]string{
		"magazine": {"weapon", "gun", "firearm", "military", "tactical", "shooting", "hunting"},
		"clip":     {"weapon", "gun", "firearm", "military", "tactical", "shooting", "ammunition"},
		"stock":    {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
		"scope":    {"weapon", "gun", "firearm", "military", "tactical", "shooting", "hunting"},
		"trigger":  {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
		"barrel":   {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
	}

	// 检测各类违禁品，但对敏感词汇进行上下文验证
	d.detectWeaponsWithContext(productTexts, categories, contextSensitiveWords, result)
	d.detectDrugs(productTexts, result)
	d.detectAdultContent(productTexts, result)
	d.detectCounterfeit(productTexts, result)
	d.detectDangerous(productTexts, result)
	d.detectMedical(productTexts, result)
	d.detectTobacco(productTexts, result)
	d.detectLiveAnimals(productTexts, result)
}

// detectWeaponsWithContext 检测武器类违禁品（带上下文验证）
func (d *ProhibitedItemsDetector) detectWeaponsWithContext(texts []string, categories []string, contextSensitiveWords map[string][]string, result *ProhibitedItemResult) {
	// 明确的武器关键词（无需上下文验证）
	definiteWeaponKeywords := []string{
		"gun", "rifle", "pistol", "ammunition", "bullet", "cartridge",
		"shotgun", "revolver", "handgun", "assault", "sniper", "silencer", "suppressor", "muzzle",
		"knife", "blade", "sword", "dagger", "machete", "bayonet", "tactical knife",
		"airsoft", "bb gun", "pellet gun", "replica gun", "toy gun", "fake gun",
		"1911", "ar-15", "ak-47", "glock", "beretta", "smith wesson", "colt",
		"枪", "步枪", "手枪", "火器", "武器", "弹药", "子弹", "弹夹", "刀具", "刀片",
		"枪支配件", "枪械配件", "握把", "枪托", "枪管", "瞄准镜",
	}

	// 需要上下文验证的关键词（包括firearm和weapon）
	contextSensitiveKeywords := []string{
		"magazine", "clip", "stock", "scope", "trigger", "barrel", "holster",
		"firearm", "weapon", // 添加这两个需要上下文验证的词
	}

	// 检查明确的武器关键词
	d.checkKeywords(texts, definiteWeaponKeywords, "武器类", result)

	// 检查需要上下文验证的关键词
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, keyword := range contextSensitiveKeywords {
			if strings.Contains(lowerText, strings.ToLower(keyword)) {
				// 检查是否在武器相关的上下文中
				if d.isWeaponContext(texts, categories, contextSensitiveWords[keyword]) {
					result.ViolatedItems = append(result.ViolatedItems, keyword)
					if result.ViolatedCategory == "" {
						result.ViolatedCategory = "武器类"
					}
					d.logger.Debugf("🔍 检测到上下文相关的武器关键词: %s", keyword)
				} else {
					d.logger.Infof("✅ 关键词 '%s' 不在武器上下文中，跳过。产品可能是: %v", keyword, categories)
				}
			}
		}
	}

	// 检查正则模式
	weaponPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(gun|rifle|pistol)\b`),
		regexp.MustCompile(`(?i)\b(ammunition|bullet)\b`),
		regexp.MustCompile(`(?i)\b(knife|blade|sword|dagger)\b`),
		regexp.MustCompile(`(?i)\b(tactical\s+knife|combat\s+knife)\b`),
		regexp.MustCompile(`(?i)\b(airsoft|bb\s*gun|pellet\s*gun)\b`),
		regexp.MustCompile(`(?i)\b(replica\s*gun|toy\s*gun|fake\s*gun)\b`),
	}

	d.checkPatterns(texts, weaponPatterns, "武器类", result)
}

// isWeaponContext 检查是否在武器相关的上下文中
func (d *ProhibitedItemsDetector) isWeaponContext(texts []string, categories []string, weaponContextWords []string) bool {
	// 合并所有文本进行检查
	allTexts := append(texts, categories...)

	// 首先检查是否为安全设备（保险箱、保险柜等）
	safetyDeviceKeywords := []string{
		"safe", "safes", "vault", "security cabinet", "lock box", "lockbox",
		"storage cabinet", "gun safe", "fireproof safe", "wall safe", "floor safe",
		"保险箱", "保险柜", "安全柜", "储物柜",
	}

	for _, text := range allTexts {
		lowerText := strings.ToLower(text)
		for _, safetyKeyword := range safetyDeviceKeywords {
			if strings.Contains(lowerText, strings.ToLower(safetyKeyword)) {
				d.logger.Infof("✅ 检测到安全设备关键词: %s，这是合法的安全存储设备", safetyKeyword)
				return false // 安全设备不是武器
			}
		}
	}

	// 检查是否在安全相关的分类中
	safetyCategories := []string{
		"safety & security", "safes", "safe accessories", "security", "home security",
		"office security", "storage & organization", "storage solutions",
	}

	for _, category := range categories {
		lowerCategory := strings.ToLower(category)
		for _, safetyCategory := range safetyCategories {
			if strings.Contains(lowerCategory, strings.ToLower(safetyCategory)) {
				d.logger.Infof("✅ 检测到安全设备分类: %s，这是合法的安全存储设备", category)
				return false // 安全设备分类不是武器
			}
		}
	}

	// 如果不是安全设备，再检查武器上下文
	for _, text := range allTexts {
		lowerText := strings.ToLower(text)
		for _, contextWord := range weaponContextWords {
			if strings.Contains(lowerText, strings.ToLower(contextWord)) {
				d.logger.Debugf("🔍 发现武器上下文关键词: %s", contextWord)
				return true
			}
		}
	}

	// 检查是否在高风险分类中
	highRiskCategories := []string{
		"hunting", "fishing", "airsoft", "military", "tactical", "outdoor", "sports",
		"toy weapons", "toy figures", "playsets", "hunting & fishing",
		"outdoor recreation", "airsoft", "toy weapons",
	}

	for _, category := range categories {
		lowerCategory := strings.ToLower(category)
		for _, riskCategory := range highRiskCategories {
			if strings.Contains(lowerCategory, strings.ToLower(riskCategory)) {
				d.logger.Debugf("🔍 发现高风险分类: %s", category)
				return true
			}
		}
	}

	return false
}

// detectWeapons 检测武器类违禁品
func (d *ProhibitedItemsDetector) detectWeapons(texts []string, result *ProhibitedItemResult) {
	weaponKeywords := []string{
		"gun", "rifle", "pistol", "firearm", "weapon", "ammunition", "bullet", "cartridge",
		"shotgun", "revolver", "handgun", "assault", "sniper", "scope", "trigger", "barrel",
		"magazine", "clip", "holster", "silencer", "suppressor", "muzzle", "stock",
		"knife", "blade", "sword", "dagger", "machete", "bayonet", "tactical",
		"airsoft", "bb gun", "pellet gun", "replica gun", "toy gun", "fake gun",
		"枪", "步枪", "手枪", "火器", "武器", "弹药", "子弹", "弹夹", "刀具", "刀片",
	}

	weaponPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(gun|rifle|pistol|firearm)\b`),
		regexp.MustCompile(`(?i)\b(weapon|ammunition|bullet)\b`),
		regexp.MustCompile(`(?i)\b(knife|blade|sword|dagger)\b`),
		regexp.MustCompile(`(?i)\b(tactical|military|combat)\b`),
		regexp.MustCompile(`(?i)\b(airsoft|bb\s*gun|pellet\s*gun)\b`),
		regexp.MustCompile(`(?i)\b(replica\s*gun|toy\s*gun|fake\s*gun)\b`),
	}

	d.checkKeywords(texts, weaponKeywords, "武器类", result)
	d.checkPatterns(texts, weaponPatterns, "武器类", result)
}

// detectDrugs 检测毒品类违禁品
func (d *ProhibitedItemsDetector) detectDrugs(texts []string, result *ProhibitedItemResult) {
	drugKeywords := []string{
		"drug", "narcotic", "cocaine", "heroin", "marijuana", "cannabis", "opium",
		"methamphetamine", "ecstasy", "lsd", "mdma", "steroid", "anabolic",
		"prescription", "controlled substance", "illegal drug",
		"毒品", "大麻", "可卡因", "海洛因", "鸦片", "兴奋剂", "违禁药物",
	}

	drugPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(drug|narcotic|cocaine|heroin)\b`),
		regexp.MustCompile(`(?i)\b(marijuana|cannabis|weed)\b`),
		regexp.MustCompile(`(?i)\b(steroid|anabolic)\b`),
		regexp.MustCompile(`(?i)\b(controlled\s*substance)\b`),
	}

	d.checkKeywords(texts, drugKeywords, "毒品类", result)
	d.checkPatterns(texts, drugPatterns, "毒品类", result)
}

// detectAdultContent 检测成人内容违禁品（优化版）
func (d *ProhibitedItemsDetector) detectAdultContent(texts []string, result *ProhibitedItemResult) {
	// 白名单：成人尺码等正常商品
	adultSizeWhitelist := []string{
		"adult size", "adult clothing", "adult shoes", "adult apparel", "adult wear",
		"men's adult", "women's adult", "adult unisex", "adult fit",
		"成人尺码", "成人服装", "成人鞋子",
	}

	// 检查是否为成人尺码商品
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, whitelistItem := range adultSizeWhitelist {
			if strings.Contains(lowerText, strings.ToLower(whitelistItem)) {
				d.logger.Infof("✅ 检测到成人尺码关键词: %s，这是合法的成人尺码商品", whitelistItem)
				return // 直接返回，不标记为违禁品
			}
		}
	}

	// 只检测真正的成人内容
	adultKeywords := []string{
		"adult content", "adult entertainment", "sex", "porn", "erotic", "sexual", "intimate", "xxx",
		"vibrator", "dildo", "condom", "lubricant", "adult toy", "sex toy", "adult video", "adult film",
		"成人内容", "成人娱乐", "性", "色情", "情趣", "性用品", "成人影片",
	}

	adultPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(adult\s*(content|entertainment|video|film))\b`),
		regexp.MustCompile(`(?i)\b(sex|porn|erotic)\b`),
		regexp.MustCompile(`(?i)\b(sexual|intimate|xxx)\b`),
		regexp.MustCompile(`(?i)\b(adult\s*toy|sex\s*toy)\b`),
	}

	d.checkKeywords(texts, adultKeywords, "成人内容", result)
	d.checkPatterns(texts, adultPatterns, "成人内容", result)
}

// detectCounterfeit 检测假冒伪劣品（优化版）
func (d *ProhibitedItemsDetector) detectCounterfeit(texts []string, result *ProhibitedItemResult) {
	// 白名单：正常的复制品/备份相关词汇
	legitimateCopyWhitelist := []string{
		"copy paper", "backup copy", "carbon copy", "hard copy", "soft copy",
		"copy machine", "copy holder", "copy stand", "photocopy",
		"replica model", "replica statue", "replica artwork", "historical replica",
		"复印纸", "备份", "复印机", "模型复制品", "艺术复制品",
	}

	// 检查是否为合法的复制品
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, whitelistItem := range legitimateCopyWhitelist {
			if strings.Contains(lowerText, strings.ToLower(whitelistItem)) {
				d.logger.Infof("✅ 检测到合法复制品关键词: %s，这是合法商品", whitelistItem)
				return // 直接返回，不标记为违禁品
			}
		}
	}

	// 只检测明确的假冒品牌商品
	counterfeits := []string{
		"replica weapon", "replica gun", "fake brand", "counterfeit brand", "imitation brand", "knockoff brand",
		"1:1 replica", "aaa quality replica", "super copy brand", "mirror quality brand",
		"仿制品牌", "假冒品牌", "山寨品牌", "高仿品牌", "A货品牌", "原单品牌",
	}

	counterfeitsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(replica\s*(weapon|gun|brand))\b`),
		regexp.MustCompile(`(?i)\b(fake|counterfeit)\s*(brand|designer|luxury)\b`),
		regexp.MustCompile(`(?i)\b(imitation|knockoff)\s*(brand|designer)\b`),
		regexp.MustCompile(`(?i)\b(1:1|aaa\s*quality)\s*(replica|copy)\b`),
		regexp.MustCompile(`(?i)\b(super\s*copy|mirror\s*quality)\s*(brand|designer)\b`),
	}

	d.checkKeywords(texts, counterfeits, "假冒伪劣", result)
	d.checkPatterns(texts, counterfeitsPatterns, "假冒伪劣", result)
}

// detectDangerous 检测危险品（优化版）
func (d *ProhibitedItemsDetector) detectDangerous(texts []string, result *ProhibitedItemResult) {
	// 白名单：正常的化学用品
	chemicalWhitelist := []string{
		"cleaning chemical", "household chemical", "cosmetic chemical", "food chemical",
		"chemical peel", "chemical exfoliant", "hair chemical", "nail chemical",
		"pool chemical", "garden chemical", "automotive chemical",
		"清洁剂", "化妆品", "护肤品", "洗发水", "指甲油",
	}

	// 检查是否为合法的化学用品
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, whitelistItem := range chemicalWhitelist {
			if strings.Contains(lowerText, strings.ToLower(whitelistItem)) {
				d.logger.Infof("✅ 检测到合法化学用品关键词: %s，这是合法商品", whitelistItem)
				return // 直接返回，不标记为违禁品
			}
		}
	}

	// 只检测真正的危险品
	dangerousKeywords := []string{
		"explosive", "bomb", "grenade", "dynamite", "fireworks", "flammable liquid", "flammable gas",
		"toxic substance", "poison", "dangerous chemical", "radioactive", "hazardous material", "corrosive acid",
		"爆炸物", "炸弹", "烟花", "易燃液体", "有毒物质", "危险化学品", "危险品", "腐蚀性酸",
	}

	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(explosive|bomb|grenade)\b`),
		regexp.MustCompile(`(?i)\b(toxic\s*substance|poison)\b`),
		regexp.MustCompile(`(?i)\b(dangerous\s*chemical|hazardous\s*material)\b`),
		regexp.MustCompile(`(?i)\b(flammable\s*(liquid|gas))\b`),
		regexp.MustCompile(`(?i)\b(corrosive\s*acid)\b`),
		regexp.MustCompile(`(?i)\b(radioactive)\b`),
	}

	d.checkKeywords(texts, dangerousKeywords, "危险品", result)
	d.checkPatterns(texts, dangerousPatterns, "危险品", result)
}

// detectMedical 检测医疗器械（优化版）
func (d *ProhibitedItemsDetector) detectMedical(texts []string, result *ProhibitedItemResult) {
	// 白名单：非处方医疗用品和保健品
	medicalWhitelist := []string{
		"vitamin", "supplement", "health supplement", "dietary supplement",
		"first aid", "bandage", "thermometer", "blood pressure monitor",
		"massage", "fitness", "wellness", "health monitor",
		"维生素", "保健品", "营养品", "急救包", "绷带", "体温计", "按摩",
	}

	// 检查是否为合法的医疗保健用品
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, whitelistItem := range medicalWhitelist {
			if strings.Contains(lowerText, strings.ToLower(whitelistItem)) {
				d.logger.Infof("✅ 检测到合法医疗保健用品关键词: %s，这是合法商品", whitelistItem)
				return // 直接返回，不标记为违禁品
			}
		}
	}

	// 只检测真正的医疗器械和处方药
	medicalKeywords := []string{
		"medical device", "prescription medicine", "prescription drug", "pharmaceutical drug", "syringe",
		"needle", "scalpel", "surgical instrument", "diagnostic equipment", "therapeutic device",
		"医疗器械", "处方药", "处方药品", "注射器", "手术刀", "医用器械", "诊断设备",
	}

	medicalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(medical\s*device|prescription)\b`),
		regexp.MustCompile(`(?i)\b(pharmaceutical\s*drug|prescription\s*(medicine|drug))\b`),
		regexp.MustCompile(`(?i)\b(surgical\s*instrument|diagnostic\s*equipment|therapeutic\s*device)\b`),
	}

	d.checkKeywords(texts, medicalKeywords, "医疗器械", result)
	d.checkPatterns(texts, medicalPatterns, "医疗器械", result)
}

// detectTobacco 检测烟草制品
func (d *ProhibitedItemsDetector) detectTobacco(texts []string, result *ProhibitedItemResult) {
	tobaccoKeywords := []string{
		"cigarette", "tobacco", "cigar", "smoking", "nicotine", "vape", "e-cigarette",
		"hookah", "pipe tobacco", "chewing tobacco",
		"香烟", "烟草", "雪茄", "尼古丁", "电子烟", "水烟",
	}

	tobaccoPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(cigarette|tobacco|cigar)\b`),
		regexp.MustCompile(`(?i)\b(vape|e-cigarette|smoking)\b`),
		regexp.MustCompile(`(?i)\b(nicotine|hookah)\b`),
	}

	d.checkKeywords(texts, tobaccoKeywords, "烟草制品", result)
	d.checkPatterns(texts, tobaccoPatterns, "烟草制品", result)
}

// detectLiveAnimals 检测活体动物（优化版）
func (d *ProhibitedItemsDetector) detectLiveAnimals(texts []string, result *ProhibitedItemResult) {
	// 白名单：明确的宠物用品关键词，不应被标记为违禁品
	petSupplyWhitelist := []string{
		"pet supplies", "pet accessories", "pet food", "pet toy", "pet bed", "pet carrier",
		"dog supplies", "cat supplies", "dog food", "cat food", "dog toy", "cat toy",
		"dog leash", "dog collar", "dog harness", "cat litter", "pet grooming",
		"dog bowl", "cat bowl", "pet shampoo", "pet brush", "pet crate",
		"宠物用品", "宠物食品", "宠物玩具", "狗粮", "猫粮", "狗绳", "猫砂",
	}

	// 检查是否为宠物用品
	for _, text := range texts {
		lowerText := strings.ToLower(text)
		for _, whitelistItem := range petSupplyWhitelist {
			if strings.Contains(lowerText, strings.ToLower(whitelistItem)) {
				d.logger.Infof("✅ 检测到宠物用品关键词: %s，这是合法的宠物用品", whitelistItem)
				return // 直接返回，不标记为违禁品
			}
		}
	}

	// 只检测明确的活体动物销售
	liveAnimalKeywords := []string{
		"live animal", "live pet", "puppy for sale", "kitten for sale", "live bird", "live fish", "live reptile",
		"exotic animal", "wildlife", "endangered species", "breeding animal",
		"活体动物", "活体宠物", "小狗出售", "小猫出售", "活鸟", "活鱼", "爬虫", "野生动物", "繁殖动物",
	}

	// 使用更精确的正则模式
	liveAnimalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(live\s*(animal|pet|dog|cat|bird|fish|reptile))\b`),
		regexp.MustCompile(`(?i)\b(puppy|kitten|bird|fish)\s*(for\s*sale|breeding|live)\b`),
		regexp.MustCompile(`(?i)\b(exotic\s*animal|wildlife|endangered\s*species)\b`),
		regexp.MustCompile(`(?i)\b(breeding\s*(animal|dog|cat|bird))\b`),
	}

	d.checkKeywords(texts, liveAnimalKeywords, "活体动物", result)
	d.checkPatterns(texts, liveAnimalPatterns, "活体动物", result)
}

// checkKeywords 检查关键词
func (d *ProhibitedItemsDetector) checkKeywords(texts []string, keywords []string, category string, result *ProhibitedItemResult) {
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

// checkPatterns 检查正则模式
func (d *ProhibitedItemsDetector) checkPatterns(texts []string, patterns []*regexp.Regexp, category string, result *ProhibitedItemResult) {
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

// calculateConfidence 计算置信度（优化版）
func (d *ProhibitedItemsDetector) calculateConfidence(violatedItems []string) float64 {
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

	d.logger.Debugf("置信度计算: 高=%d, 中=%d, 低=%d, 最终置信度=%.2f",
		highCount, mediumCount, lowCount, confidence)

	return confidence
}

// loadConfig 加载违禁品配置
func (d *ProhibitedItemsDetector) loadConfig() error {
	data, err := os.ReadFile(d.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config ProhibitedItemsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 加载静态关键词
	d.staticKeywords = config.StaticKeywords
	d.categoryKeywords = config.CategoryKeywords

	// 编译动态模式
	for category, patterns := range config.DynamicPatterns {
		d.dynamicPatterns[category] = []*regexp.Regexp{}
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				d.logger.WithError(err).Warnf("编译正则表达式失败: %s", pattern)
				continue
			}
			d.dynamicPatterns[category] = append(d.dynamicPatterns[category], re)
		}
	}

	d.logger.Infof("加载违禁品配置成功: 静态关键词=%d, 动态模式=%d",
		len(d.staticKeywords), len(d.dynamicPatterns))

	return nil
}

// loadDefaultConfig 加载默认配置
func (d *ProhibitedItemsDetector) loadDefaultConfig() {
	d.logger.Info("使用默认违禁品配置")
	// 默认配置已在各个检测方法中定义
}
