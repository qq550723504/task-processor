// Package sku 提供TEMU平台的SKU变体处理功能
package sku

import (
	"fmt"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/spec"

	"github.com/sirupsen/logrus"
)

// SkuVariantProcessor SKU变体处理器
type SkuVariantProcessor struct {
	logger           *logrus.Entry
	aiClient         openai.ChatCompleter
	specHandler      *SkuSpecHandler
	skcBuilder       *SkuSkcBuilder
	mappingProcessor *SkuMappingProcessor
	specResolver     *spec.SpecResolverService
	itemBuilder      *SkuItemBuilder
}

// NewSkuVariantProcessor 创建新的SKU变体处理器
func NewSkuVariantProcessor(logger *logrus.Entry, aiClient openai.ChatCompleter, specHandler *SkuSpecHandler, skcBuilder *SkuSkcBuilder, specResolver *spec.SpecResolverService, itemBuilder *SkuItemBuilder) *SkuVariantProcessor {
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
	skcList, err := vp.BuildSkcsFromAIMapping(temuCtx, variants, temuCtx.AISkuMapping)
	if err != nil {
		vp.logger.Warnf("根据AI映射构建SKC失败: %v，使用默认映射", err)
		return nil
	}

	temuCtx.TemuProduct.SkcList = skcList
	vp.logger.Infof("AI辅助构建完成，创建了%d个SKC", len(skcList))
	return nil
}

// buildSkcsFromAIMapping 根据AI映射构建SKC
func (vp *SkuVariantProcessor) BuildSkcsFromAIMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiMapping *temucontext.AISkuMappingResponse) ([]models.Skc, error) {
	// 检查AI映射数量
	if aiMapping.SkuCount() != len(variants) {
		vp.logger.Warnf("⚠️ AI映射数量(%d)与变体数量(%d)不匹配", aiMapping.SkuCount(), len(variants))

		// 使用映射处理器修复数量不匹配问题
		if err := vp.mappingProcessor.FixMappingCountMismatch(aiMapping, variants); err != nil {
			return nil, fmt.Errorf("修复映射数量不匹配失败: %w", err)
		}
	}

	if err := vp.normalizeAIMappingForBuild(temuCtx, aiMapping); err != nil {
		return nil, err
	}

	// 检查规格来源：只有GoodsSpecProperties不为空且有预置规格值时，才创建多SKC
	templateInfo := temuCtx.TemplateInfo
	hasTemplateInfo := templateInfo != nil
	userInputSpecs := temuCtx.UserInputParentSpecList
	hasUserInputSpecs := len(userInputSpecs) > 0

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
		var templateSpecs []temutemplate.TemplateRespGoodsSpecProperty
		if hasUserInputSpecs && len(userInputSpecs) > 0 {
			vp.logger.Infof("使用UserInputParentSpecList，用户规格数量: %d", len(userInputSpecs))
			templateSpecs = vp.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		} else if hasTemplateInfo {
			vp.logger.Infof("使用GoodsSpecProperties（无预置值），规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
			templateSpecs = templateInfo.GoodsSpecProperties
		} else {
			vp.logger.Warn("未找到任何规格信息")
			templateSpecs = []temutemplate.TemplateRespGoodsSpecProperty{}
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
	var aiMapping *temucontext.AISkuMappingResponse
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
		aiMapping, err = vp.GenerateAISkuMapping(temuCtx, variants)
		if err != nil {
			vp.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
			return models.Skc{}, fmt.Errorf("AI生成SKU映射失败: %w", err)
		}
	}

	if aiMapping.SkuCount() == 0 {
		return models.Skc{}, fmt.Errorf("AI未生成任何SKU")
	}

	// 使用第一个AI生成的SKU
	aiSku, ok := aiMapping.FirstSKU()
	if !ok {
		return models.Skc{}, fmt.Errorf("AI?????????")
	}

	if err := vp.normalizeAIMappingForBuild(temuCtx, aiMapping); err != nil {
		return models.Skc{}, err
	}

	vp.logger.Infof("✅ AI成功生成规格: %+v", aiSku.Spec)
	vp.logger.Infof("✅ AI提取的重量尺寸: weight=%s, length=%s, width=%s, height=%s",
		aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)

	// 使用AI生成的SKU构建完整的SKU
	sku := vp.itemBuilder.buildSkuFromVariantWithAI(temuCtx, amazonProduct, *aiSku)

	return models.Skc{
		SkuList: []models.Sku{sku},
	}, nil
}

// generateAISkuMapping 生成AI SKU映射
func (vp *SkuVariantProcessor) GenerateAISkuMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) (*temucontext.AISkuMappingResponse, error) {
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
	response, err := vp.GenerateAISkuMappingSingleBatch(temuCtx, variants)
	if err != nil {
		return nil, err
	}

	// 重新启用规格维度统一器，但改进逻辑以处理混合属性情况
	// AI可能仍然会为混合属性选择不同维度，需要统一处理
	if err := vp.normalizeGeneratedAIMapping(response); err != nil {
		return nil, err
	}

	return response, nil
}

func (vp *SkuVariantProcessor) validateAIMappingSpecs(aiMapping *temucontext.AISkuMappingResponse) error {
	var validationErr error
	aiMapping.ForEachSKUIndexed(func(i int, aiSku *temucontext.AIGeneratedSku) {
		if validationErr != nil {
			return
		}
		if len(aiSku.Spec) > 2 {
			vp.logger.Errorf("AI mapping[%d] has too many specs: %d", i, len(aiSku.Spec))
			vp.logger.Errorf("Specs: %+v", aiSku.Spec)
			validationErr = fmt.Errorf("AI mapping[%d] has too many specs: %d", i, len(aiSku.Spec))
			return
		}

		if err := vp.specHandler.ValidateSpecs(convertSpecInfos(aiSku.Spec)); err != nil {
			vp.logger.Errorf("AI mapping[%d] spec validation failed: %v", i, err)
			vp.logger.Error("AI generated specs do not match TEMU spec constraints")
			validationErr = fmt.Errorf("AI mapping[%d] spec validation failed: %w", i, err)
			return
		}

		vp.logger.Infof("SKU[%d] AI dimensions: weight=%s, length=%s, width=%s, height=%s",
			i, aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)
	})

	return validationErr
}

func (vp *SkuVariantProcessor) resolveAIMappingSpecIDs(temuCtx *temucontext.TemuTaskContext, aiMapping *temucontext.AISkuMappingResponse) error {
	resolveRuntime, err := spec.BuildResolveSpecRuntimeInput(temuCtx)
	if err != nil {
		return fmt.Errorf("build spec resolve runtime: %w", err)
	}
	if err := vp.specResolver.ResolveTemporarySpecIDs(resolveRuntime, aiMapping); err != nil {
		vp.logger.Errorf("failed to resolve temporary spec IDs: %v", err)
		return fmt.Errorf("resolve temporary spec IDs: %w", err)
	}

	vp.logger.Info("Resolved all temporary spec IDs")
	return nil
}

func (vp *SkuVariantProcessor) normalizeAIMappingForBuild(temuCtx *temucontext.TemuTaskContext, aiMapping *temucontext.AISkuMappingResponse) error {
	if err := vp.validateAIMappingSpecs(aiMapping); err != nil {
		return err
	}
	if err := vp.resolveAIMappingSpecIDs(temuCtx, aiMapping); err != nil {
		return err
	}

	return nil
}

func (vp *SkuVariantProcessor) normalizeGeneratedAIMapping(aiMapping *temucontext.AISkuMappingResponse) error {
	if err := vp.unifyAIMappingSpecDimensions(aiMapping); err != nil {
		vp.logger.Errorf("single-batch spec dimension unify failed: %v", err)
		return fmt.Errorf("single-batch spec dimension unify: %w", err)
	}

	return nil
}

func (vp *SkuVariantProcessor) unifyAIMappingSpecDimensions(aiMapping *temucontext.AISkuMappingResponse) error {
	unifier := spec.NewSpecDimensionUnifier()
	return unifier.UnifySpecDimensions(aiMapping)
}
