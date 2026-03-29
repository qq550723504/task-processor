package filter

import (
	"fmt"

	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ProhibitedItemsDetector TEMU违禁品检测器
type ProhibitedItemsDetector struct {
	logger              *logrus.Entry
	configLoader        *ConfigLoader
	utils               *DetectorUtils
	weaponsDetector     *WeaponsDetector
	drugsDetector       *DrugsDetector
	adultDetector       *AdultContentDetector
	counterfeitDetector *CounterfeitDetector
	dangerousDetector   *DangerousDetector
	medicalDetector     *MedicalDetector
	tobaccoDetector     *TobaccoDetector
	animalsDetector     *LiveAnimalsDetector
}

// NewProhibitedItemsDetector 创建违禁品检测器
func NewProhibitedItemsDetector() *ProhibitedItemsDetector {
	logger := logger.GetGlobalLogger("ProhibitedItemsDetector")

	// 创建配置加载器
	configLoader := NewConfigLoader(logger, "data/prohibited_items_temu.json")

	// 尝试加载配置
	_, err := configLoader.LoadConfig()
	if err != nil {
		logger.WithError(err).Warn("加载违禁品配置失败，使用默认配置")
		configLoader.LoadDefaultConfig()
	}

	// 创建工具类
	utils := NewDetectorUtils(logger)

	// 创建各个检测器
	weaponsDetector := NewWeaponsDetector(logger, utils)
	drugsDetector := NewDrugsDetector(logger, utils)
	adultDetector := NewAdultContentDetector(logger, utils)
	counterfeitDetector := NewCounterfeitDetector(logger, utils)
	dangerousDetector := NewDangerousDetector(logger, utils)
	medicalDetector := NewMedicalDetector(logger, utils)
	tobaccoDetector := NewTobaccoDetector(logger, utils)
	animalsDetector := NewLiveAnimalsDetector(logger, utils)

	return &ProhibitedItemsDetector{
		logger:              logger,
		configLoader:        configLoader,
		utils:               utils,
		weaponsDetector:     weaponsDetector,
		drugsDetector:       drugsDetector,
		adultDetector:       adultDetector,
		counterfeitDetector: counterfeitDetector,
		dangerousDetector:   dangerousDetector,
		medicalDetector:     medicalDetector,
		tobaccoDetector:     tobaccoDetector,
		animalsDetector:     animalsDetector,
	}
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
		d.logProductDetails(amazonProduct)

		// 返回不可重试错误（使用 NONRETRYABLE: 前缀，temu.IsRetryableError 会识别）
		return fmt.Errorf("NONRETRYABLE: 产品包含违禁品内容: %s (类别: %s), 违禁品检测失败: %v",
			result.Reason, result.ViolatedCategory, result.ViolatedItems,
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
func (d *ProhibitedItemsDetector) DetectProhibitedItems(amazonProduct *model.Product) *ProhibitedItemResult {
	result := &ProhibitedItemResult{
		IsProhibited:  false,
		ViolatedItems: []string{},
		Confidence:    0.0,
	}

	// 提取产品文本信息（标题）
	productTexts := d.utils.ExtractProductTexts(amazonProduct)
	if len(productTexts) == 0 {
		d.logger.Warn("⚠️ 没有可检测的产品文本，跳过违禁词检测")
		return result
	}

	// 提取产品分类信息
	categories := d.utils.ExtractProductCategories(amazonProduct)

	// 首先检查是否为明确的合法产品类别
	if d.utils.IsLegitimateProductCategory(categories, productTexts) {
		d.logger.Info("✅ 检测到合法产品类别，跳过违禁品检测")
		return result
	}

	// 使用各个检测器进行检测
	d.weaponsDetector.Detect(productTexts, categories, result)
	d.drugsDetector.Detect(productTexts, result)
	d.adultDetector.Detect(productTexts, result)
	d.counterfeitDetector.Detect(productTexts, result)
	d.dangerousDetector.Detect(productTexts, result)
	d.medicalDetector.Detect(productTexts, result)
	d.tobaccoDetector.Detect(productTexts, result)
	d.animalsDetector.Detect(productTexts, result)

	// 计算总体置信度
	if len(result.ViolatedItems) > 0 {
		result.Confidence = d.utils.CalculateConfidence(result.ViolatedItems)

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

// logProductDetails 打印产品详细信息
func (d *ProhibitedItemsDetector) logProductDetails(amazonProduct *model.Product) {
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
			if i < 5 {
				d.logger.Errorf("     - %s", feature)
			}
		}
	}

	// 打印产品详情（可能包含违禁词）
	if len(amazonProduct.ProductDetails) > 0 {
		d.logger.Errorf("   Amazon产品详情:")
		for i, detail := range amazonProduct.ProductDetails {
			if i < 5 {
				d.logger.Errorf("     %s: %s", detail.Type, detail.Value)
			}
		}
	}
}
