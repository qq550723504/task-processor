package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/pipeline"
	management_api "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"
)

// ProductSubmitHandler 产品提交处理器
type ProductSubmitHandler struct {
	*BaseTemuHandler
	saveHandler   *ProductSaveHandler
	mappingClient management_api.ProductImportMappingAPI
	errorAnalyzer *ProductSubmitErrorAnalyzer
	utils         *ProductSubmitUtils
}

// NewProductSubmitHandler 创建新的产品提交处理器
func NewProductSubmitHandler(mappingClient management_api.ProductImportMappingAPI) *ProductSubmitHandler {
	baseHandler := NewBaseTemuHandler("产品提交处理器")
	return &ProductSubmitHandler{
		BaseTemuHandler: baseHandler,
		saveHandler:     NewProductSaveHandler(),
		mappingClient:   mappingClient,
		errorAnalyzer:   NewProductSubmitErrorAnalyzer(baseHandler.logger),
		utils:           NewProductSubmitUtils(baseHandler.logger),
	}
}

// Name 返回处理器名称
func (h *ProductSubmitHandler) Name() string {
	return "产品提交处理器"
}

// ensureProperFormatting 确保产品名称格式正确（提交前的最后检查）
func (h *ProductSubmitHandler) ensureProperFormatting(name string) string {
	// 1. 确保左括号前有空格（TEMU要求）
	name = regexp.MustCompile(`(\S)\(`).ReplaceAllString(name, "$1 (")

	// 2. 确保右括号后有空格（如果后面还有字符）
	name = regexp.MustCompile(`\)(\S)`).ReplaceAllString(name, ") $1")

	// 3. 移除逗号前的空格
	name = regexp.MustCompile(`\s+,`).ReplaceAllString(name, ",")

	// 4. 移除其他标点符号前的空格
	name = regexp.MustCompile(`\s+([.!?;:])`).ReplaceAllString(name, "$1")

	// 5. 清理多余的空格
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	return name
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *ProductSubmitHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始提交产品")

	// 检查任务上下文中的必要数据
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 提交产品（产品存在性检查已在第3步完成）
	err := h.submitProduct(temuCtx)
	if err != nil {
		h.logger.Errorf("提交产品失败: %v", err)
		return fmt.Errorf("提交产品失败: %w", err)
	}

	h.logger.Info("产品提交完成")
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *ProductSubmitHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// submitProduct 提交产品
func (h *ProductSubmitHandler) submitProduct(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始提交产品到TEMU")

	temuProduct := temuCtx.TemuProduct

	// 【最后的格式检查】在提交前确保产品名称格式正确
	if temuProduct.GoodsBasic.GoodsName != "" {
		originalName := temuProduct.GoodsBasic.GoodsName
		fixedName := h.ensureProperFormatting(originalName)

		if fixedName != originalName {
			h.logger.Warnf("⚠️ 提交前修复产品名称格式: '%s' -> '%s'", originalName, fixedName)
			temuProduct.GoodsBasic.GoodsName = fixedName
		}
	}

	// 【规格完整性验证】确保所有必需的规格都已配置
	if err := h.validateSpecCompleteness(temuCtx); err != nil {
		h.logger.Errorf("规格完整性验证失败: %v", err)
		return fmt.Errorf("规格配置不完整: %w", err)
	}

	// 【多件套包装验证】确保多件套产品的包装配置正确
	if err := h.validateMultiplePackageConfiguration(temuCtx); err != nil {
		h.logger.Errorf("多件套包装配置验证失败: %v", err)
		return fmt.Errorf("多件套包装配置错误: %w", err)
	}

	// 构造TEMU产品提交请求
	request := h.buildSubmitRequest(temuCtx)

	// 序列化请求用于调试和错误处理
	requestJSON, jsonErr := h.utils.MarshalWithoutHTMLEscape(request)
	if jsonErr != nil {
		h.logger.Errorf("序列化请求JSON失败: %v", jsonErr)
		return fmt.Errorf("序列化请求失败: %w", jsonErr)
	}

	// 保存JSON到文件用于调试
	task := temuCtx.GetTask()
	if task != nil {
		taskID := fmt.Sprintf("%d", task.ID)
		if saveErr := h.utils.SaveJSONToFile(taskID, requestJSON, task.ProductID); saveErr != nil {
			h.logger.Errorf("保存JSON文件失败: %v", saveErr)
		}
	}

	// 创建SubmitAPI
	submitAPI := api.NewSubmitAPI(temuCtx.APIClient, h.logger)

	// 调用API提交产品
	response, err := submitAPI.SubmitProduct(request)
	if err != nil {
		// 保存JSON数据到文件用于调试
		task := temuCtx.GetTask()
		if task != nil {
			taskID := fmt.Sprintf("%d", task.ID)
			if saveErr := h.utils.SaveJSONToFile(taskID, requestJSON, "product_submit"); saveErr != nil {
				h.logger.Errorf("保存JSON文件失败: %v", saveErr)
			}
		}
		h.logger.Errorf("产品提交失败: %v", err)
		return fmt.Errorf("产品提交失败: %w", err)
	}

	h.logger.Infof("out_goods_sn: %s", request.GoodsBasic.OutGoodsSN)

	// 检查响应结果
	if !response.Success {
		h.logger.Errorf("TEMU API响应失败: success=%v, error_code=%d, error_message=%v", response.Success, response.ErrorCode, response.Message)

		// 🔍 简单记录错误信息用于分析
		h.errorAnalyzer.AnalyzeError(temuCtx, response.ErrorCode, response.Message)

		// 检查是否为不可重试的错误
		if h.utils.IsNonRetryableError(response.ErrorCode, response.Message) {
			h.logger.Errorf("❌ 检测到不可重试错误(error_code=%d): %s", response.ErrorCode, response.Message)
			h.logger.Error("❌ 此错误无法通过重试解决，任务将被标记为失败")
			// 返回特殊错误，让上层知道这是不可重试的
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d): %s", response.ErrorCode, response.Message)
		}

		// 可重试的错误，尝试保存到草稿箱
		h.logger.Warnf("产品提交失败，尝试保存到草稿箱...")
		if saveErr := h.saveHandler.Handle(temuCtx); saveErr != nil {
			h.logger.Errorf("保存到草稿箱也失败: %v", saveErr)
			h.logger.Error("❌ 提交和保存草稿都失败，任务将被标记为不可重试")
			// 提交失败且保存草稿也失败，标记为不可重试
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d)且保存草稿失败: %w", response.ErrorCode, saveErr)
		}
		h.logger.Infof("✅ 产品已保存到草稿箱，任务标记为已完成")
		// 保存到草稿箱成功，标记为特殊的成功状态，避免重复处理
		temuCtx.SavedToDraft = true
		return nil // 返回nil表示处理成功，不再重试
	}

	// 保存提交响应到强类型字段
	temuCtx.SubmitResult = response

	h.logger.Infof("🎉 产品发布成功！商品ID: %s, 商品名称: %s", temuProduct.GoodsBasic.GoodsID, temuProduct.GoodsBasic.GoodsName)

	return nil
}

// buildSubmitRequest 构建提交请求
func (h *ProductSubmitHandler) buildSubmitRequest(temuCtx *temucontext.TemuTaskContext) *models.ProductSubmitRequest {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct

	// 转换Extra类型
	extra := models.Extra{
		Tab:              temuProduct.Extra.Tab,
		MinSkuImageSize:  temuProduct.Extra.MinSkuImageSize,
		MaxSkuImageSize:  temuProduct.Extra.MaxSkuImageSize,
		CreateEmptyGoods: temuProduct.Extra.CreateEmptyGoods,
	}

	request := &models.ProductSubmitRequest{
		GoodsBasic:            temuProduct.GoodsBasic,
		GoodsSaleInfo:         temuProduct.GoodsSaleInfo,
		GoodsServicePromise:   temuProduct.GoodsServicePromise,
		GoodsExtensionInfo:    temuProduct.GoodsExtensionInfo,
		Extra:                 extra,
		CanSave:               true,
		SupportMaxRetailPrice: true,
		PlatformExpressBill:   false,
		SkcList:               temuProduct.SkcList,
		//BatchSkuInfo:          batchSkuInfo,
	}

	h.logger.Infof("构建提交请求完成: SKC数量=%d, 总SKU数量=%d",
		len(request.SkcList), h.utils.GetTotalSkuCount(request.SkcList))

	return request
}

// validateSpecCompleteness 验证规格完整性
func (h *ProductSubmitHandler) validateSpecCompleteness(temuCtx *temucontext.TemuTaskContext) error {
	temuProduct := temuCtx.TemuProduct

	// 收集所有SKU使用的规格维度
	specDimensions := make(map[string]map[string]bool) // parent_spec_id -> spec_id -> exists

	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if specDimensions[spec.ParentSpecID] == nil {
					specDimensions[spec.ParentSpecID] = make(map[string]bool)
				}
				specDimensions[spec.ParentSpecID][spec.SpecID] = true
			}
		}
	}

	// 验证规格属性配置是否包含所有使用的规格，并检查关键字段
	configuredSpecs := make(map[string]map[string]bool)   // parent_spec_id -> spec_id -> exists
	incompleteSpecs := make(map[string]map[string]string) // parent_spec_id -> spec_id -> missing_field

	for _, specProp := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
		if configuredSpecs[specProp.ParentSpecID] == nil {
			configuredSpecs[specProp.ParentSpecID] = make(map[string]bool)
			incompleteSpecs[specProp.ParentSpecID] = make(map[string]string)
		}
		configuredSpecs[specProp.ParentSpecID][specProp.SpecID] = true

		// 检查关键字段是否缺失
		if specProp.Vid == 0 {
			incompleteSpecs[specProp.ParentSpecID][specProp.SpecID] = "missing_vid"
			h.logger.Warnf("🔍 规格配置缺少vid字段: %s(%s) -> %s(%s)",
				specProp.ParentSpecName, specProp.ParentSpecID, specProp.Value, specProp.SpecID)
		}
	}

	// 检查是否有缺失的规格配置或不完整的规格
	var missingSpecs []string
	var incompleteSpecsList []string

	for parentSpecID, specIDs := range specDimensions {
		for specID := range specIDs {
			if configuredSpecs[parentSpecID] == nil || !configuredSpecs[parentSpecID][specID] {
				// 查找规格名称
				specName := h.findSpecName(temuCtx, parentSpecID, specID)
				missingSpecs = append(missingSpecs, fmt.Sprintf("%s(%s)", specName, specID))
			} else if incompleteSpecs[parentSpecID] != nil && incompleteSpecs[parentSpecID][specID] != "" {
				// 规格存在但不完整
				specName := h.findSpecName(temuCtx, parentSpecID, specID)
				incompleteSpecsList = append(incompleteSpecsList, fmt.Sprintf("%s(%s)-%s", specName, specID, incompleteSpecs[parentSpecID][specID]))
			}
		}
	}

	if len(missingSpecs) > 0 || len(incompleteSpecsList) > 0 {
		if len(missingSpecs) > 0 {
			h.logger.Errorf("🔍 检测到缺失的规格配置: %v", missingSpecs)
		}
		if len(incompleteSpecsList) > 0 {
			h.logger.Errorf("🔍 检测到不完整的规格配置: %v", incompleteSpecsList)
		}
		h.logger.Error("🔍 这可能导致TEMU API返回'reset the variants template'错误")

		// 尝试自动修复规格配置
		if err := h.autoFixSpecConfiguration(temuCtx, specDimensions); err != nil {
			h.logger.Errorf("自动修复规格配置失败: %v", err)
			return fmt.Errorf("缺失或不完整的规格配置且无法自动修复: missing=%v, incomplete=%v", missingSpecs, incompleteSpecsList)
		}

		h.logger.Info("✅ 已自动修复规格配置")
	}

	return nil
}

// findSpecName 查找规格名称
func (h *ProductSubmitHandler) findSpecName(temuCtx *temucontext.TemuTaskContext, parentSpecID, specID string) string {
	// 从AI映射中查找
	if temuCtx.AISkuMapping != nil {
		for _, aiSku := range temuCtx.AISkuMapping.SkuList {
			for _, spec := range aiSku.Spec {
				if spec.ParentSpecID == parentSpecID && spec.SpecID == specID {
					return spec.SpecName
				}
			}
		}
	}

	// 从SKU中查找
	for _, skc := range temuCtx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if spec.ParentSpecID == parentSpecID && spec.SpecID == specID {
					return spec.SpecName
				}
			}
		}
	}

	return fmt.Sprintf("Unknown_%s", specID)
}

// autoFixSpecConfiguration 自动修复规格配置
func (h *ProductSubmitHandler) autoFixSpecConfiguration(temuCtx *temucontext.TemuTaskContext, specDimensions map[string]map[string]bool) error {
	temuProduct := temuCtx.TemuProduct

	// 获取模板信息用于验证
	templateInfo, hasTemplate := GetTemplateInfoFromContext(temuCtx)

	// 首先修复现有规格的缺失字段
	for i := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
		specProp := &temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties[i]

		// 如果vid缺失，尝试从模板中获取
		if specProp.Vid == 0 && hasTemplate {
			if vid := h.findVidFromTemplate(templateInfo, specProp.ParentSpecID, specProp.SpecID); vid > 0 {
				specProp.Vid = vid
				h.logger.Infof("🔧 已为规格配置添加vid: %s(%s) -> %s(%s) vid=%d",
					specProp.ParentSpecName, specProp.ParentSpecID, specProp.Value, specProp.SpecID, vid)
			}
		}

		// 如果template相关字段缺失，尝试从模板中获取
		if specProp.TemplatePid == 0 && hasTemplate {
			if templatePid := h.findTemplatePidFromTemplate(templateInfo, specProp.ParentSpecID); templatePid > 0 {
				specProp.TemplatePid = templatePid
			}
		}

		if specProp.TemplateModuleID == 0 && hasTemplate {
			if moduleId := h.findTemplateModuleIdFromTemplate(templateInfo, specProp.ParentSpecID); moduleId > 0 {
				specProp.TemplateModuleID = moduleId
			}
		}
	}

	// 然后添加缺失的规格配置

	for parentSpecID, specIDs := range specDimensions {
		for specID := range specIDs {
			// 检查是否已配置
			found := false
			for _, existing := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties {
				if existing.ParentSpecID == parentSpecID && existing.SpecID == specID {
					found = true
					break
				}
			}

			if !found {
				// 查找规格信息
				specName := h.findSpecName(temuCtx, parentSpecID, specID)
				parentSpecName := h.findParentSpecName(temuCtx, parentSpecID)

				// 创建规格属性配置
				specProp := models.GoodSpecProperty{
					Value:          specName,
					SpecID:         specID,
					ParentSpecID:   parentSpecID,
					ParentSpecName: parentSpecName,
					Checked:        true,
					ControlType:    0,
					Disabled:       false,
					Name:           parentSpecName,
					IsCustomized:   1,
				}

				// 如果有模板信息，尝试获取vid和template相关字段
				if hasTemplate {
					if vid := h.findVidFromTemplate(templateInfo, parentSpecID, specID); vid > 0 {
						specProp.Vid = vid
					}
					if templatePid := h.findTemplatePidFromTemplate(templateInfo, parentSpecID); templatePid > 0 {
						specProp.TemplatePid = templatePid
					}
					if moduleId := h.findTemplateModuleIdFromTemplate(templateInfo, parentSpecID); moduleId > 0 {
						specProp.TemplateModuleID = moduleId
					}
				}

				// 添加到规格属性列表
				temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties = append(
					temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsSpecProperties,
					specProp,
				)

				h.logger.Infof("🔧 已添加缺失的规格配置: %s(%s) -> %s(%s)",
					parentSpecName, parentSpecID, specName, specID)
			}
		}
	}

	return nil
}

// findParentSpecName 查找父规格名称
func (h *ProductSubmitHandler) findParentSpecName(temuCtx *temucontext.TemuTaskContext, parentSpecID string) string {
	// 从AI映射中查找
	if temuCtx.AISkuMapping != nil {
		for _, aiSku := range temuCtx.AISkuMapping.SkuList {
			for _, spec := range aiSku.Spec {
				if spec.ParentSpecID == parentSpecID {
					return spec.ParentSpecName
				}
			}
		}
	}

	// 从SKU中查找
	for _, skc := range temuCtx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			for _, spec := range sku.Spec {
				if spec.ParentSpecID == parentSpecID {
					return spec.ParentSpecName
				}
			}
		}
	}

	return fmt.Sprintf("Unknown_%s", parentSpecID)
}

// findVidFromTemplate 从模板中查找vid
func (h *ProductSubmitHandler) findVidFromTemplate(templateInfo *types.TemplateInfo, parentSpecID, specID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			for _, value := range specProp.Values {
				if value.SpecID == specID {
					return value.VID
				}
			}
		}
	}
	return 0
}

// findTemplatePidFromTemplate 从模板中查找template_pid
func (h *ProductSubmitHandler) findTemplatePidFromTemplate(templateInfo *types.TemplateInfo, parentSpecID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			return specProp.TemplatePID
		}
	}
	return 0
}

// findTemplateModuleIdFromTemplate 从模板中查找template_module_id
func (h *ProductSubmitHandler) findTemplateModuleIdFromTemplate(templateInfo *types.TemplateInfo, parentSpecID string) int {
	for _, specProp := range templateInfo.GoodsSpecProperties {
		if specProp.ParentSpecID == parentSpecID {
			return specProp.TemplateModuleID
		}
	}
	return 0
}

// validateMultiplePackageConfiguration 验证多件套包装配置
func (h *ProductSubmitHandler) validateMultiplePackageConfiguration(temuCtx *temucontext.TemuTaskContext) error {
	temuProduct := temuCtx.TemuProduct

	// 检查产品属性中是否设置为多件套
	isMultipleSets := false
	var multipleSetsPropIndex = -1
	for i, prop := range temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties {
		if strings.Contains(strings.ToLower(prop.Value), "multiple sets") {
			isMultipleSets = true
			multipleSetsPropIndex = i
			h.logger.Infof("🔍 检测到产品属性设置为多件套: %s", prop.Value)
			break
		}
	}

	if !isMultipleSets {
		// 不是多件套产品，无需验证
		return nil
	}

	// 检查所有SKU的包装配置是否正确
	hasInvalidPackaging := false
	for skcIndex, skc := range temuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			mp := sku.MultiplePackage

			// 多件套产品的包装数量必须大于1
			if mp.NumberOfPieces <= 1 {
				h.logger.Warnf("🔧 检测到多件套产品但包装数量为1 (SKC:%d, SKU:%d)", skcIndex, skuIndex)
				hasInvalidPackaging = true
				break
			}
		}
		if hasInvalidPackaging {
			break
		}
	}

	// 如果包装配置不正确，将产品属性改为单件
	if hasInvalidPackaging {
		h.logger.Warnf("🔧 多件套配置不正确，将产品属性修改为单件")

		// 直接删除"Multiple Sets"属性
		if multipleSetsPropIndex >= 0 {
			temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties = append(
				temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties[:multipleSetsPropIndex],
				temuProduct.GoodsExtensionInfo.GoodsProperty.GoodsProperties[multipleSetsPropIndex+1:]...)
			h.logger.Infof("🔧 已删除'Multiple Sets'属性")
		}

		// 确保所有SKU的包装配置为单件
		for _, skc := range temuProduct.SkcList {
			for _, sku := range skc.SkuList {
				sku.MultiplePackage.SkuClassification = 1 // 单品
				sku.MultiplePackage.NumberOfPieces = 1
				sku.MultiplePackage.NumberOfPiecesNew = "1"
				sku.MultiplePackage.IndividuallyPacked = 1
			}
		}

		h.logger.Info("✅ 已将产品配置修改为单件")
	}

	return nil
}
