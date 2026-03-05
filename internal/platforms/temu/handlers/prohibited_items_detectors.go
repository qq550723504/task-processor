package handlers

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// DrugsDetector 毒品检测器
type DrugsDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewDrugsDetector 创建毒品检测器
func NewDrugsDetector(logger *logrus.Entry, utils *DetectorUtils) *DrugsDetector {
	return &DrugsDetector{logger: logger, utils: utils}
}

// Detect 检测毒品类违禁品
func (d *DrugsDetector) Detect(texts []string, result *ProhibitedItemResult) {
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

	d.utils.CheckKeywords(texts, drugKeywords, "毒品类", result)
	d.utils.CheckPatterns(texts, drugPatterns, "毒品类", result)
}

// AdultContentDetector 成人内容检测器
type AdultContentDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewAdultContentDetector 创建成人内容检测器
func NewAdultContentDetector(logger *logrus.Entry, utils *DetectorUtils) *AdultContentDetector {
	return &AdultContentDetector{logger: logger, utils: utils}
}

// Detect 检测成人内容违禁品
func (d *AdultContentDetector) Detect(texts []string, result *ProhibitedItemResult) {
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
				return
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

	d.utils.CheckKeywords(texts, adultKeywords, "成人内容", result)
	d.utils.CheckPatterns(texts, adultPatterns, "成人内容", result)
}

// CounterfeitDetector 假冒伪劣检测器
type CounterfeitDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewCounterfeitDetector 创建假冒伪劣检测器
func NewCounterfeitDetector(logger *logrus.Entry, utils *DetectorUtils) *CounterfeitDetector {
	return &CounterfeitDetector{logger: logger, utils: utils}
}

// Detect 检测假冒伪劣品
func (d *CounterfeitDetector) Detect(texts []string, result *ProhibitedItemResult) {
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
				return
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

	d.utils.CheckKeywords(texts, counterfeits, "假冒伪劣", result)
	d.utils.CheckPatterns(texts, counterfeitsPatterns, "假冒伪劣", result)
}

// DangerousDetector 危险品检测器
type DangerousDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewDangerousDetector 创建危险品检测器
func NewDangerousDetector(logger *logrus.Entry, utils *DetectorUtils) *DangerousDetector {
	return &DangerousDetector{logger: logger, utils: utils}
}

// Detect 检测危险品
func (d *DangerousDetector) Detect(texts []string, result *ProhibitedItemResult) {
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
				return
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

	d.utils.CheckKeywords(texts, dangerousKeywords, "危险品", result)
	d.utils.CheckPatterns(texts, dangerousPatterns, "危险品", result)
}

// MedicalDetector 医疗器械检测器
type MedicalDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewMedicalDetector 创建医疗器械检测器
func NewMedicalDetector(logger *logrus.Entry, utils *DetectorUtils) *MedicalDetector {
	return &MedicalDetector{logger: logger, utils: utils}
}

// Detect 检测医疗器械
func (d *MedicalDetector) Detect(texts []string, result *ProhibitedItemResult) {
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
				return
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

	d.utils.CheckKeywords(texts, medicalKeywords, "医疗器械", result)
	d.utils.CheckPatterns(texts, medicalPatterns, "医疗器械", result)
}

// TobaccoDetector 烟草检测器
type TobaccoDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewTobaccoDetector 创建烟草检测器
func NewTobaccoDetector(logger *logrus.Entry, utils *DetectorUtils) *TobaccoDetector {
	return &TobaccoDetector{logger: logger, utils: utils}
}

// Detect 检测烟草制品
func (d *TobaccoDetector) Detect(texts []string, result *ProhibitedItemResult) {
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

	d.utils.CheckKeywords(texts, tobaccoKeywords, "烟草制品", result)
	d.utils.CheckPatterns(texts, tobaccoPatterns, "烟草制品", result)
}

// LiveAnimalsDetector 活体动物检测器
type LiveAnimalsDetector struct {
	logger *logrus.Entry
	utils  *DetectorUtils
}

// NewLiveAnimalsDetector 创建活体动物检测器
func NewLiveAnimalsDetector(logger *logrus.Entry, utils *DetectorUtils) *LiveAnimalsDetector {
	return &LiveAnimalsDetector{logger: logger, utils: utils}
}

// Detect 检测活体动物
func (d *LiveAnimalsDetector) Detect(texts []string, result *ProhibitedItemResult) {
	// 白名单：明确的宠物用品关键词
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
				return
			}
		}
	}

	// 只检测明确的活体动物销售
	liveAnimalKeywords := []string{
		"live animal", "live pet", "puppy for sale", "kitten for sale", "live bird", "live fish", "live reptile",
		"exotic animal", "wildlife", "endangered species", "breeding animal",
		"活体动物", "活体宠物", "小狗出售", "小猫出售", "活鸟", "活鱼", "爬虫", "野生动物", "繁殖动物",
	}

	liveAnimalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(live\s*(animal|pet|dog|cat|bird|fish|reptile))\b`),
		regexp.MustCompile(`(?i)\b(puppy|kitten|bird|fish)\s*(for\s*sale|breeding|live)\b`),
		regexp.MustCompile(`(?i)\b(exotic\s*animal|wildlife|endangered\s*species)\b`),
		regexp.MustCompile(`(?i)\b(breeding\s*(animal|dog|cat|bird))\b`),
	}

	d.utils.CheckKeywords(texts, liveAnimalKeywords, "活体动物", result)
	d.utils.CheckPatterns(texts, liveAnimalPatterns, "活体动物", result)
}
