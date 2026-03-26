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
	if err := vp.prepareAIMappingForBuild(temuCtx, variants, aiMapping); err != nil {
		return nil, err
	}

	return vp.buildSkcsFromPreparedAIMapping(temuCtx, variants, aiMapping), nil
}

func (vp *SkuVariantProcessor) buildSkcsFromPreparedAIMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiMapping *temucontext.AISkuMappingResponse) []models.Skc {
	templateSpecs, shouldCreateMultipleSkcs := vp.resolveSkcBuildSpecs(temuCtx)
	if shouldCreateMultipleSkcs {
		return vp.buildMultipleSkcs(temuCtx, variants, aiMapping, templateSpecs)
	}

	return vp.buildSingleSkc(temuCtx, variants, aiMapping, templateSpecs)
}

func (vp *SkuVariantProcessor) resolveSkcBuildSpecs(temuCtx *temucontext.TemuTaskContext) ([]temutemplate.TemplateRespGoodsSpecProperty, bool) {
	templateInfo := temuCtx.TemplateInfo
	hasTemplateInfo := templateInfo != nil
	userInputSpecs := temuCtx.UserInputParentSpecList
	hasUserInputSpecs := len(userInputSpecs) > 0

	if hasTemplateInfo && len(templateInfo.GoodsSpecProperties) > 0 {
		for _, prop := range templateInfo.GoodsSpecProperties {
			if len(prop.Values) > 0 {
				vp.logger.Infof("GoodsSpecProperties有预置规格值，构建多个SKC，规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
				return templateInfo.GoodsSpecProperties, true
			}
		}
	}

	if hasUserInputSpecs && len(userInputSpecs) > 0 {
		vp.logger.Infof("使用UserInputParentSpecList，用户规格数量: %d", len(userInputSpecs))
		return vp.specHandler.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs), false
	} else if hasTemplateInfo {
		vp.logger.Infof("使用GoodsSpecProperties（无预置值），规格属性数量: %d", len(templateInfo.GoodsSpecProperties))
		return templateInfo.GoodsSpecProperties, false
	}

	vp.logger.Warn("未找到任何规格信息")
	return []temutemplate.TemplateRespGoodsSpecProperty{}, false
}

func (vp *SkuVariantProcessor) buildMultipleSkcs(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	aiMapping *temucontext.AISkuMappingResponse,
	templateSpecs []temutemplate.TemplateRespGoodsSpecProperty,
) []models.Skc {
	return vp.skcBuilder.buildMultipleSkcsFromTemplate(temuCtx, variants, aiMapping, templateSpecs)
}

func (vp *SkuVariantProcessor) buildSingleSkc(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	aiMapping *temucontext.AISkuMappingResponse,
	templateSpecs []temutemplate.TemplateRespGoodsSpecProperty,
) []models.Skc {
	vp.logger.Info("创建单个SKC，多个SKU")
	return vp.skcBuilder.buildSingleSkcFromUserInput(temuCtx, variants, aiMapping, templateSpecs)
}

// CreateDefaultSkc 创建默认SKC（用于没有变体的产品）
func (vp *SkuVariantProcessor) CreateDefaultSkc(temuCtx *temucontext.TemuTaskContext) (models.Skc, error) {
	vp.logger.Info("创建默认SKC（产品没有变体）")

	// 直接从强类型上下文获取Amazon产品信息
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		return models.Skc{}, fmt.Errorf("没有Amazon产品信息")
	}

	aiMapping, err := vp.prepareDefaultAIMappingForBuild(temuCtx, amazonProduct)
	if err != nil {
		return models.Skc{}, err
	}

	return vp.buildDefaultSkcFromPreparedMapping(temuCtx, amazonProduct, aiMapping)
}

func (vp *SkuVariantProcessor) buildDefaultSkcFromPreparedMapping(temuCtx *temucontext.TemuTaskContext, amazonProduct *model.Product, aiMapping *temucontext.AISkuMappingResponse) (models.Skc, error) {
	aiSku, err := vp.selectDefaultAIMapping(aiMapping)
	if err != nil {
		return models.Skc{}, err
	}
	sku := vp.buildDefaultSkuFromAIMapping(temuCtx, amazonProduct, aiSku)

	return models.Skc{
		SkuList: []models.Sku{sku},
	}, nil
}

func (vp *SkuVariantProcessor) selectDefaultAIMapping(aiMapping *temucontext.AISkuMappingResponse) (*temucontext.AIGeneratedSku, error) {
	if aiMapping.SkuCount() == 0 {
		return nil, fmt.Errorf("AI未生成任何SKU")
	}

	aiSku, ok := aiMapping.FirstSKU()
	if !ok {
		return nil, fmt.Errorf("AI?????????")
	}

	return aiSku, nil
}

func (vp *SkuVariantProcessor) buildDefaultSkuFromAIMapping(
	temuCtx *temucontext.TemuTaskContext,
	amazonProduct *model.Product,
	aiSku *temucontext.AIGeneratedSku,
) models.Sku {
	vp.logger.Infof("✅ AI成功生成规格: %+v", aiSku.Spec)
	vp.logger.Infof("✅ AI提取的重量尺寸: weight=%s, length=%s, width=%s, height=%s",
		aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)

	return vp.itemBuilder.buildSkuFromVariantWithAI(temuCtx, amazonProduct, *aiSku)
}

func (vp *SkuVariantProcessor) resolveDefaultAIMapping(temuCtx *temucontext.TemuTaskContext, amazonProduct *model.Product) (*temucontext.AISkuMappingResponse, error) {
	if temuCtx.AISkuMapping != nil {
		vp.logger.Info("✅ 使用AISkuMappingHandler已生成的AI映射，避免重复调用")
		return temuCtx.AISkuMapping, nil
	}

	vp.logger.Info("未找到现有AI映射，开始生成新的AI映射")
	variants := []*model.Product{amazonProduct}
	aiMapping, err := vp.GenerateAISkuMapping(temuCtx, variants)
	if err != nil {
		vp.logger.Errorf("❌ AI生成SKU映射失败: %v", err)
		return nil, fmt.Errorf("AI生成SKU映射失败: %w", err)
	}

	return aiMapping, nil
}

func (vp *SkuVariantProcessor) prepareDefaultAIMappingForBuild(temuCtx *temucontext.TemuTaskContext, amazonProduct *model.Product) (*temucontext.AISkuMappingResponse, error) {
	aiMapping, err := vp.resolveDefaultAIMapping(temuCtx, amazonProduct)
	if err != nil {
		return nil, err
	}

	variants := []*model.Product{amazonProduct}
	if err := vp.prepareAIMappingForBuild(temuCtx, variants, aiMapping); err != nil {
		return nil, err
	}

	return aiMapping, nil
}

// generateAISkuMapping 生成AI SKU映射
func (vp *SkuVariantProcessor) GenerateAISkuMapping(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) (*temucontext.AISkuMappingResponse, error) {
	if vp.aiClient == nil {
		return nil, fmt.Errorf("AI客户端未初始化")
	}

	if err := vp.validateAIMappingVariantCount(variants); err != nil {
		return nil, err
	}

	// 根据token限制决定是否需要分批处理
	// Gemini 2.0 Flash输出限制约8000 tokens，每个SKU约400 tokens
	// 安全起见，每批最多处理20个变体
	const maxVariantsPerBatch = 20

	return vp.generateAIMappingByVariantCount(temuCtx, variants, maxVariantsPerBatch)
}

func (vp *SkuVariantProcessor) validateAIMappingVariantCount(variants []*model.Product) error {
	if len(variants) <= 100 {
		return nil
	}

	vp.logger.Errorf("❌ 变体数量超过限制: %d > 100，系统无法处理如此多的变体", len(variants))
	vp.logger.Error("❌ 此错误不应重试，请检查产品数据或联系技术支持")
	return fmt.Errorf("变体数量超过限制: %d > 100，系统无法处理", len(variants))
}

func (vp *SkuVariantProcessor) generateAIMappingByVariantCount(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	maxVariantsPerBatch int,
) (*temucontext.AISkuMappingResponse, error) {
	if len(variants) > maxVariantsPerBatch {
		vp.logger.Infof("🔧 变体数量(%d)超过单批限制(%d)，将分批处理", len(variants), maxVariantsPerBatch)
		return vp.generateAISkuMappingInBatches(temuCtx, variants, maxVariantsPerBatch)
	}

	return vp.generateAISkuMappingSingle(temuCtx, variants)
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

func (vp *SkuVariantProcessor) prepareAIMappingForBuild(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, aiMapping *temucontext.AISkuMappingResponse) error {
	if err := vp.ensureAIMappingMatchesVariants(aiMapping, variants); err != nil {
		return err
	}

	return vp.normalizeAIMappingForBuild(temuCtx, aiMapping)
}

func (vp *SkuVariantProcessor) ensureAIMappingMatchesVariants(aiMapping *temucontext.AISkuMappingResponse, variants []*model.Product) error {
	if aiMapping.SkuCount() == len(variants) {
		return nil
	}

	vp.logger.Warnf("⚠️ AI映射数量(%d)与变体数量(%d)不匹配", aiMapping.SkuCount(), len(variants))
	if err := vp.mappingProcessor.FixMappingCountMismatch(aiMapping, variants); err != nil {
		return fmt.Errorf("修复映射数量不匹配失败: %w", err)
	}

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

func (vp *SkuVariantProcessor) finalizeGeneratedAIMapping(aiMapping *temucontext.AISkuMappingResponse) (*temucontext.AISkuMappingResponse, error) {
	if err := vp.normalizeGeneratedAIMapping(aiMapping); err != nil {
		return nil, err
	}

	return aiMapping, nil
}

func (vp *SkuVariantProcessor) generateAISkuMappingSingle(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) (*temucontext.AISkuMappingResponse, error) {
	response, err := vp.GenerateAISkuMappingSingleBatch(temuCtx, variants)
	if err != nil {
		return nil, err
	}

	return vp.finalizeGeneratedAIMapping(response)
}

func (vp *SkuVariantProcessor) unifyAIMappingSpecDimensions(aiMapping *temucontext.AISkuMappingResponse) error {
	unifier := spec.NewSpecDimensionUnifier()
	return unifier.UnifySpecDimensions(aiMapping)
}
