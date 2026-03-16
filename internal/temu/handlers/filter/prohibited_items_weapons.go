package filter

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// WeaponsDetector 武器检测器
type WeaponsDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewWeaponsDetector 创建武器检测器
func NewWeaponsDetector(logger *logrus.Entry, utils *DetectorUtils) *WeaponsDetector {
	return &WeaponsDetector{
		logger: logger,
		utils:  utils,
	}
}

// Detect 检测武器类违禁品
func (d *WeaponsDetector) Detect(texts []string, categories []string, result *ProhibitedItemResult) {
	// 定义需要上下文验证的敏感词汇
	contextSensitiveWords := map[string][]string{
		"magazine": {"weapon", "gun", "firearm", "military", "tactical", "shooting", "hunting"},
		"clip":     {"weapon", "gun", "firearm", "military", "tactical", "shooting", "ammunition"},
		"stock":    {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
		"scope":    {"weapon", "gun", "firearm", "military", "tactical", "shooting", "hunting"},
		"trigger":  {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
		"barrel":   {"weapon", "gun", "firearm", "military", "tactical", "shooting"},
	}

	d.detectWeaponsWithContext(texts, categories, contextSensitiveWords, result)
}

// detectWeaponsWithContext 检测武器类违禁品（带上下文验证）
func (d *WeaponsDetector) detectWeaponsWithContext(texts []string, categories []string, contextSensitiveWords map[string][]string, result *ProhibitedItemResult) {
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
		"firearm", "weapon",
	}

	// 检查明确的武器关键词
	d.utils.CheckKeywords(texts, definiteWeaponKeywords, "武器类", result)

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

	d.utils.CheckPatterns(texts, weaponPatterns, "武器类", result)
}

// isWeaponContext 检查是否在武器相关的上下文中
func (d *WeaponsDetector) isWeaponContext(texts []string, categories []string, weaponContextWords []string) bool {
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
				return false
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
				return false
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
