// Package handlers 提供TEMU平台的SKU变体处理功能
package handlers

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// SkuVariantProcessor SKU变体处理器
type SkuVariantProcessor struct {
	logger           *logrus.Entry
	aiClient         *openai.Client
	specHandler      *SkuSpecHandler
	skcBuilder       *SkuSkcBuilder
	mappingProcessor *SkuMappingProcessor
	specResolver     *SpecResolverService
	itemBuilder      *SkuItemBuilder
}

// NewSkuVariantProcessor 创建新的SKU变体处理器
func NewSkuVariantProcessor(logger *logrus.Entry, aiClient *openai.Client, specHandler *SkuSpecHandler, skcBuilder *SkuSkcBuilder, specResolver *SpecResolverService, itemBuilder *SkuItemBuilder) *SkuVariantProcessor {
	return &SkuVariantProcessor{
		logger:           logger,
		aiClient:         aiClient,
		specHandler:      specHandler,
		skcBuilder:       skcBuilder,
		mappingProcessor: NewSkuMappingProcessor(logger, specHandler),
		specResolver:     specResolver,
		itemBuilder:      itemBuilder,
	}
}

// BuildVariantSkcs 构建变体SKC
func (vp *SkuVariantProcessor) BuildVariantSkcs(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) error {
	vp.logger.Infof("构建变体SKC，变体数量: %d", len(variants))

	// 根据AI映射构建SKC列表
	skcList, err := vp.buildSkcsFromAIMapping(temuCtx, variants, temuCtx.AISkuMapping)
	if err != nil {
		vp.logger.Warnf("根据AI映射构建SKC失败: %v，使用默认映射", err)
		return nil
	}

	temuCtx.TemuProduct.SkcList = skcList
	vp.logger.Infof("AI辅助构建完成，创建了%d个SKC", len(skcList))
	return nil
}

// buildSkcsFromAIMapping 根据AI映射构建SKC
func (vp *SkuVariantProcessor) buildSkcsFromAIMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiMapping *types.AISkuMappingResponse) ([]models.Skc, error) {
	// 检查AI映射数量
	if len(aiMapping.SkuList) != len(variants) {
		vp.logger.Warnf("⚠️ AI映射数量(%d)与变体数量(%d)不匹配", len(aiMapping.SkuList), len(variants))

		// 使用映射处理器修复数量不匹配问题
		if err := vp.mappingProcessor.FixMappingCountMismatch(aiMapping, variants); err != nil {
			return nil, fmt.Errorf("修复映射数量不匹配失败: %w", err)
		}
	}

	// 预防性检查：验证AI映射中的规格数量和有效性
	for i, aiSku := range aiMapping.SkuList {
		// 检查规格数量是否超过2个（TEMU限制）
		if len(aiSku.Spec) > 2 {
			vp.logger.Errorf("❌ AI映射[%d]规格数量超限: 当前有%d个规格，TEMU最多允许2个销售规格", i, len(aiSku.Spec))
			vp.logger.Errorf("❌ 规格详情: %+v", aiSku.Spec)
			return nil, fmt.Errorf("AI映射[%d]规格数量超限: 有%d个规格，但TEMU最多允许2个", i, len(aiSku.Spec))
		}

		// 验证规格是否有效
		if err := vp.specHandler.ValidateSpecs(aiSku.Spec); err != nil {
			vp.logger.Errorf("❌ AI映射[%d]规格验证失败: %v", i, err)
			vp.logger.Error("❌ AI必须从TEMU模板中选择有效的规格，不能使用默认规格")
			return nil, fmt.Errorf("AI映射[%d]规格无效: %w", i, err)
		}
		// 输出AI提取的物流信息
		vp.logger.Infof("📦 SKU[%d] AI提取的物流信息: weight=%s, length=%s, width=%s, height=%s",
			i, aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)
	}

	// 将TemuTaskContext转换为TaskContext接口
	if err := vp.specResolver.ResolveTemporarySpecIDs(temuCtx, aiMapping); err != nil {
		vp.logger.Errorf("❌ 解析规格ID失败: %v", err)
		return nil, fmt.Errorf("解析临时规格ID失败: %w", err)
	}

	// 检查规格来源：只有GoodsSpecProperties不为空且有预置规格值时，才创建多SKC
	var templateInfo *types.TemplateInfo
	var hasTemplateInfo bool
	if temuCtx.TemplateInfo != nil {
		if info, ok := temuCtx.TemplateInfo.(*types.TemplateInfo); ok {
			templateInfo = info
			hasTemplateInfo = true
		}
	}

	var userInputSpecs []types.UserInputParentSpec
	var hasUserInputSpecs bool
	if temuCtx.UserInputParentSpecList != nil {
		if specs, ok := temuCtx.UserInputParentSpecList.([]types.UserInputParentSpec); ok {
			userInputSpecs = specs
			hasUserInputSpecs = true
		}
	}

	var skcList []models.Skc

	// 检查是否应该创建多SKC：GoodsSpecProperties不为空且有预置规格值
	shouldCreateMultipleSkcs := false
	if hasTemplateInfo && len(templateInfo.GoodsSpecProperties) > 0 {
		// 检查是否有预置的规格值
		for _, prop := range templateInfo.GoodsSpecProperties {
			if len(prop.Values) > 0 {
				shouldCreateMultipleSkcs = true
				break
			}
		}
	}

	if shouldCreateMultipleSkcs {
		// 创建多个SKC（按主变体分组）
		vp.logger.Infof("GoodsSpecProperties有预置规格值，构建多个SKC，规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
		skcList = vp.skcBuilder.buildMultipleSkcsFromTemplate(temuCtx, variants, aiMapping, templateInfo.GoodsSpecProperties)
	} else {
		// 创建单个SKC，多个SKU
		vp.logger.Info("创建单个SKC，多个SKU")

		// 优先使用UserInputParentSpecList，否则使用空的模板规格
		var templateSpecs []types.TemplateRespGoodsSpecProperty
		if hasUserInputSpecs && len(userInputSpecs) > 0 {
			vp.logger.Infof("使用UserInputParentSpecList，用户规格数量: %d", len(userInputSpecs))
			templateSpecs = vp.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		} else if hasTemplateInfo {
			vp.logger.Infof("使用GoodsSpecProperties（无预置值），规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
			templateSpecs = templateInfo.GoodsSpecProperties
		} else {
			vp.logger.Warn("未找到任何规格信息")
			templateSpecs = []types.TemplateRespGoodsSpecProperty{}
		}

		skcList = vp.skcBuilder.buildSingleSkcFromUserInput(temuCtx, variants, aiMapping, templateSpecs)
	}

	return skcList, nil
}

// CreateDefaultSkc 创建默认SKC（用于没有变体的产品）
func (vp *SkuVariantProcessor) CreateDefaultSkc(temuCtx *temucontext.TemuTaskContext) (models.Skc, error) {
	vp.logger.Info("创建默认SKC（产品没有变体）")

	// 直接从强类型上下文获取Amazon产品信息
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		return models.Skc{}, fmt.Errorf("没有Amazon产品信息")
	}

	// 优先使用AISkuMappingHandler已经生成的AI映射
	var aiMapping *types.AISkuMappingResponse
	if temuCtx.AISkuMapping != nil {
		aiMapping = temuCtx.AISkuMapping
		vp.logger.Info("✅ 使用AISkuMappingHandler已生成的AI映射，避免重复调用")
	}

	// 如果没有现有映射，才调用AI生成
	if aiMapping == nil {
		vp.logger.Info("未找到现有AI映射，开始生成新的AI映射")

		// 将单一产品包装成变体列表，让AI处理
		variants := []*model.Product{amazonProduct}

		// 使用AI生成SKU映射
		var err error
		aiMapping, err = vp.generateAISkuMapping(temuCtx, variants)
		if err != nil {
			vp.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
			return models.Skc{}, fmt.Errorf("AI生成SKU映射失败: %w", err)
		}
	}

	if len(aiMapping.SkuList) == 0 {
		return models.Skc{}, fmt.Errorf("AI未生成任何SKU")
	}

	// 使用第一个AI生成的SKU
	aiSku := aiMapping.SkuList[0]

	// 验证规格
	if err := vp.specHandler.ValidateSpecs(aiSku.Spec); err != nil {
		vp.logger.Errorf("❌ AI生成的规格验证失败: %v", err)
		return models.Skc{}, fmt.Errorf("AI生成的规格无效: %w", err)
	}

	vp.logger.Infof("✅ AI成功生成规格: %+v", aiSku.Spec)
	vp.logger.Infof("✅ AI提取的重量尺寸: weight=%s, length=%s, width=%s, height=%s",
		aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)

	if err := vp.specResolver.ResolveTemporarySpecIDs(temuCtx, aiMapping); err != nil {
		vp.logger.Errorf("❌ 解析规格ID失败: %v", err)
		return models.Skc{}, fmt.Errorf("解析临时规格ID失败: %w", err)
	}
	vp.logger.Info("✅ 成功解析所有临时规格ID")

	// 使用AI生成的SKU构建完整的SKU
	sku := vp.itemBuilder.buildSkuFromVariantWithAI(temuCtx, amazonProduct, aiSku)

	return models.Skc{
		SkuList: []models.Sku{sku},
	}, nil
}

// generateAISkuMapping 生成AI SKU映射
func (vp *SkuVariantProcessor) generateAISkuMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) (*types.AISkuMappingResponse, error) {
	if vp.aiClient == nil {
		return nil, fmt.Errorf("AI客户端未初始化")
	}

	// // 检查变体数量限制（超过100个变体无法处理，不应重试）
	if len(variants) > 100 {
		vp.logger.Errorf("❌ 变体数量超过限制: %d > 100，系统无法处理如此多的变体", len(variants))
		vp.logger.Error("❌ 此错误不应重试，请检查产品数据或联系技术支持")
		return nil, fmt.Errorf("变体数量超过限制: %d > 100，系统无法处理", len(variants))
	}

	// 根据token限制决定是否需要分批处理
	// Gemini 2.0 Flash输出限制约8000 tokens，每个SKU约400 tokens
	// 安全起见，每批最多处理20个变体
	const maxVariantsPerBatch = 20

	if len(variants) > maxVariantsPerBatch {
		vp.logger.Infof("🔧 变体数量(%d)超过单批限制(%d)，将分批处理", len(variants), maxVariantsPerBatch)
		return vp.generateAISkuMappingInBatches(temuCtx, variants, maxVariantsPerBatch)
	}

	// 单批处理
	response, err := vp.generateAISkuMappingSingleBatch(temuCtx, variants)
	if err != nil {
		return nil, err
	}

	// 重新启用规格维度统一器，但改进逻辑以处理混合属性情况
	// AI可能仍然会为混合属性选择不同维度，需要统一处理
	unifier := NewSpecDimensionUnifier()
	if err := unifier.UnifySpecDimensions(response); err != nil {
		vp.logger.Errorf("❌ 单批处理规格维度统一失败: %v", err)
		return nil, fmt.Errorf("规格维度统一失败: %w", err)
	}

	return response, nil
}
