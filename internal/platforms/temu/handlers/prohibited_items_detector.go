package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

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

// DetectProhibitedItems 检测违禁品
func (d *ProhibitedItemsDetector) DetectProhibitedItems(amazonProduct interface{}) *ProhibitedItemResult {
	result := &ProhibitedItemResult{
		IsProhibited:  false,
		ViolatedItems: []string{},
		Confidence:    0.0,
	}

	// 提取产品文本信息
	productTexts := d.extractProductTexts(amazonProduct)

	// 检测各类违禁品
	d.detectWeapons(productTexts, result)
	d.detectDrugs(productTexts, result)
	d.detectAdultContent(productTexts, result)
	d.detectCounterfeit(productTexts, result)
	d.detectDangerous(productTexts, result)
	d.detectMedical(productTexts, result)
	d.detectTobacco(productTexts, result)
	d.detectLiveAnimals(productTexts, result)

	// 计算总体置信度
	if len(result.ViolatedItems) > 0 {
		result.IsProhibited = true
		result.Confidence = d.calculateConfidence(result.ViolatedItems)
		result.Reason = fmt.Sprintf("检测到%d个违禁关键词", len(result.ViolatedItems))
	}

	return result
}

// extractProductTexts 提取产品文本信息
func (d *ProhibitedItemsDetector) extractProductTexts(amazonProduct interface{}) []string {
	texts := []string{}

	// 将产品数据转换为JSON字符串进行文本提取
	jsonData, err := json.Marshal(amazonProduct)
	if err != nil {
		d.logger.WithError(err).Warn("序列化产品数据失败")
		return texts
	}

	// 解析JSON获取文本字段
	var productMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &productMap); err != nil {
		d.logger.WithError(err).Warn("解析产品数据失败")
		return texts
	}

	// 递归提取所有字符串值
	d.extractStringsFromMap(productMap, &texts)

	return texts
}

// extractStringsFromMap 递归提取map中的所有字符串
func (d *ProhibitedItemsDetector) extractStringsFromMap(data map[string]interface{}, texts *[]string) {
	for _, value := range data {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				*texts = append(*texts, v)
			}
		case map[string]interface{}:
			d.extractStringsFromMap(v, texts)
		case []interface{}:
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					d.extractStringsFromMap(itemMap, texts)
				} else if itemStr, ok := item.(string); ok && strings.TrimSpace(itemStr) != "" {
					*texts = append(*texts, itemStr)
				}
			}
		}
	}
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

// detectAdultContent 检测成人内容违禁品
func (d *ProhibitedItemsDetector) detectAdultContent(texts []string, result *ProhibitedItemResult) {
	adultKeywords := []string{
		"adult", "sex", "porn", "erotic", "sexual", "intimate", "xxx",
		"vibrator", "dildo", "condom", "lubricant", "adult toy",
		"成人", "性", "色情", "情趣", "性用品",
	}

	adultPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(adult|sex|porn|erotic)\b`),
		regexp.MustCompile(`(?i)\b(sexual|intimate|xxx)\b`),
		regexp.MustCompile(`(?i)\b(adult\s*toy|sex\s*toy)\b`),
	}

	d.checkKeywords(texts, adultKeywords, "成人内容", result)
	d.checkPatterns(texts, adultPatterns, "成人内容", result)
}

// detectCounterfeit 检测假冒伪劣品
func (d *ProhibitedItemsDetector) detectCounterfeit(texts []string, result *ProhibitedItemResult) {
	counterfeits := []string{
		"replica", "fake", "counterfeit", "imitation", "knockoff", "copy",
		"authentic", "genuine", "original", "brand new", "oem",
		"仿制", "假货", "山寨", "高仿", "A货", "原单",
	}

	counterfeitsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(replica|fake|counterfeit)\b`),
		regexp.MustCompile(`(?i)\b(imitation|knockoff|copy)\b`),
		regexp.MustCompile(`(?i)\b(1:1|aaa\s*quality)\b`),
	}

	d.checkKeywords(texts, counterfeits, "假冒伪劣", result)
	d.checkPatterns(texts, counterfeitsPatterns, "假冒伪劣", result)
}

// detectDangerous 检测危险品
func (d *ProhibitedItemsDetector) detectDangerous(texts []string, result *ProhibitedItemResult) {
	dangerousKeywords := []string{
		"explosive", "bomb", "grenade", "dynamite", "fireworks", "flammable",
		"toxic", "poison", "chemical", "radioactive", "hazardous", "corrosive",
		"爆炸", "炸弹", "烟花", "易燃", "有毒", "化学品", "危险品",
	}

	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(explosive|bomb|grenade)\b`),
		regexp.MustCompile(`(?i)\b(toxic|poison|chemical)\b`),
		regexp.MustCompile(`(?i)\b(flammable|hazardous|corrosive)\b`),
	}

	d.checkKeywords(texts, dangerousKeywords, "危险品", result)
	d.checkPatterns(texts, dangerousPatterns, "危险品", result)
}

// detectMedical 检测医疗器械
func (d *ProhibitedItemsDetector) detectMedical(texts []string, result *ProhibitedItemResult) {
	medicalKeywords := []string{
		"medical device", "prescription", "medicine", "pharmaceutical", "syringe",
		"needle", "scalpel", "surgical", "diagnostic", "therapeutic",
		"医疗器械", "处方药", "药品", "注射器", "手术刀", "医用",
	}

	medicalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(medical\s*device|prescription)\b`),
		regexp.MustCompile(`(?i)\b(pharmaceutical|medicine)\b`),
		regexp.MustCompile(`(?i)\b(surgical|diagnostic|therapeutic)\b`),
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

// detectLiveAnimals 检测活体动物
func (d *ProhibitedItemsDetector) detectLiveAnimals(texts []string, result *ProhibitedItemResult) {
	animalKeywords := []string{
		"live animal", "pet", "puppy", "kitten", "bird", "fish", "reptile",
		"exotic animal", "wildlife", "endangered species",
		"活体动物", "宠物", "小狗", "小猫", "鸟类", "鱼类", "爬虫", "野生动物",
	}

	animalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(live\s*animal|pet)\b`),
		regexp.MustCompile(`(?i)\b(puppy|kitten|bird|fish)\b`),
		regexp.MustCompile(`(?i)\b(exotic\s*animal|wildlife)\b`),
	}

	d.checkKeywords(texts, animalKeywords, "活体动物", result)
	d.checkPatterns(texts, animalPatterns, "活体动物", result)
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

// calculateConfidence 计算置信度
func (d *ProhibitedItemsDetector) calculateConfidence(violatedItems []string) float64 {
	if len(violatedItems) == 0 {
		return 0.0
	}

	// 基础置信度
	baseConfidence := 0.6

	// 每个违禁项增加置信度
	itemBonus := float64(len(violatedItems)) * 0.1

	// 最大置信度为1.0
	confidence := baseConfidence + itemBonus
	if confidence > 1.0 {
		confidence = 1.0
	}

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
