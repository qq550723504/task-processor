package product

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pipeline"
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/handlerbase"

	"github.com/sirupsen/logrus"
)

// ProductSubmitHandler 产品提交处理器
type ProductSubmitHandler struct {
	*handlerbase.BaseTemuHandler
	logger        *logrus.Entry
	saveHandler   *ProductSaveHandler
	mappingClient management_api.ProductImportMappingAPI
	errorAnalyzer *ProductSubmitErrorAnalyzer
	utils         *ProductSubmitUtils
	validator     *ProductSubmitValidator
	fixer         *ProductSubmitFixer
}

// NewProductSubmitHandler 创建新的产品提交处理器
func NewProductSubmitHandler(mappingClient management_api.ProductImportMappingAPI) *ProductSubmitHandler {
	log := logger.GetGlobalLogger("temu.handlers.product_submit")

	validator := NewProductSubmitValidator(log)
	fixer := NewProductSubmitFixer(log, validator)
	baseHandler := handlerbase.NewBaseTemuHandler("product_submit")

	return &ProductSubmitHandler{
		BaseTemuHandler: baseHandler,
		logger:          log,
		saveHandler:     NewProductSaveHandler(),
		mappingClient:   mappingClient,
		errorAnalyzer:   NewProductSubmitErrorAnalyzer(log),
		utils:           NewProductSubmitUtils(log),
		validator:       validator,
		fixer:           fixer,
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
		h.logger.WithError(err).Error("提交产品失败")
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
			h.logger.WithFields(logrus.Fields{
				"original_name": originalName,
				"fixed_name":    fixedName,
			}).Warn("提交前修复产品名称格式")
			temuProduct.GoodsBasic.GoodsName = fixedName
		}
	}

	// 【规格完整性验证】确保所有必需的规格都已配置
	if err := h.validateSpecCompleteness(temuCtx); err != nil {
		h.logger.WithError(err).Error("规格完整性验证失败")
		return fmt.Errorf("规格配置不完整: %w", err)
	}

	// 【多件套包装验证】确保多件套产品的包装配置正确
	if err := h.validateMultiplePackageConfiguration(temuCtx); err != nil {
		h.logger.WithError(err).Error("多件套包装配置验证失败")
		return fmt.Errorf("多件套包装配置错误: %w", err)
	}

	// 构造TEMU产品提交请求
	request := h.buildSubmitRequest(temuCtx)

	// 序列化请求用于调试和错误处理
	requestJSON, jsonErr := h.utils.MarshalWithoutHTMLEscape(request)
	if jsonErr != nil {
		h.logger.WithError(jsonErr).Error("序列化请求JSON失败")
		return fmt.Errorf("序列化请求失败: %w", jsonErr)
	}

	// 保存JSON到文件用于调试
	task := temuCtx.GetTask()
	if task != nil {
		taskID := fmt.Sprintf("%d", task.ID)
		if saveErr := h.utils.SaveJSONToFile(taskID, requestJSON, task.ProductID); saveErr != nil {
			h.logger.WithError(saveErr).Error("保存JSON文件失败")
		}
	}

	// 创建SubmitAPI
	submitAPI := temuapi.NewSubmitAPI(temuCtx.APIClient, h.logger)

	// 调用API提交产品
	response, err := submitAPI.Submit(request)
	if err != nil {
		// 保存JSON数据到文件用于调试
		task := temuCtx.GetTask()
		if task != nil {
			taskID := fmt.Sprintf("%d", task.ID)
			if saveErr := h.utils.SaveJSONToFile(taskID, requestJSON, "product_submit"); saveErr != nil {
				h.logger.WithError(saveErr).Error("保存JSON文件失败")
			}
		}
		h.logger.WithError(err).Error("产品提交失败")
		return fmt.Errorf("产品提交失败: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"out_goods_sn": request.GoodsBasic.OutGoodsSN,
	}).Info("提交产品请求")

	// 检查响应结果
	if !response.Success {
		h.logger.WithFields(logrus.Fields{
			"success":       response.Success,
			"error_code":    response.ErrorCode,
			"error_message": response.Message,
		}).Error("TEMU API响应失败")

		// 🔍 简单记录错误信息用于分析
		h.errorAnalyzer.AnalyzeError(temuCtx, response.ErrorCode, response.Message)

		// 检查是否为不可重试的错误
		if h.utils.IsNonRetryableError(response.ErrorCode, response.Message) {
			h.logger.WithFields(logrus.Fields{
				"error_code":    response.ErrorCode,
				"error_message": response.Message,
			}).Error("检测到不可重试错误")
			h.logger.Error("此错误无法通过重试解决，任务将被标记为失败")
			// 返回特殊错误，让上层知道这是不可重试的
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d): %s", response.ErrorCode, response.Message)
		}

		// 可重试的错误，尝试保存到草稿箱
		h.logger.Warn("产品提交失败，尝试保存到草稿箱...")
		if saveErr := h.saveHandler.Handle(temuCtx); saveErr != nil {
			h.logger.WithError(saveErr).Error("保存到草稿箱也失败")
			h.logger.Error("提交和保存草稿都失败，任务将被标记为不可重试")
			// 提交失败且保存草稿也失败，标记为不可重试
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d)且保存草稿失败: %w", response.ErrorCode, saveErr)
		}
		h.logger.Info("产品已保存到草稿箱，任务标记为已完成")
		// 保存到草稿箱成功，标记为特殊的成功状态，避免重复处理
		temuCtx.SavedToDraft = true
		return nil // 返回nil表示处理成功，不再重试
	}

	// 保存提交响应到强类型字段
	temuCtx.SubmitResult = response

	h.logger.WithFields(logrus.Fields{
		"goods_id":   temuProduct.GoodsBasic.GoodsID,
		"goods_name": temuProduct.GoodsBasic.GoodsName,
	}).Info("产品发布成功")

	return nil
}

// buildSubmitRequest 构建提交请求
func (h *ProductSubmitHandler) buildSubmitRequest(temuCtx *temucontext.TemuTaskContext) *temuapi.SubmitRequest {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct

	// 转换Extra类型
	extra := temuapi.ExtraInfo{
		Tab:              temuProduct.Extra.Tab,
		MinSkuImageSize:  temuProduct.Extra.MinSkuImageSize,
		MaxSkuImageSize:  temuProduct.Extra.MaxSkuImageSize,
		CreateEmptyGoods: temuProduct.Extra.CreateEmptyGoods,
	}

	request := &temuapi.SubmitRequest{
		GoodsBasic:            temuProduct.GoodsBasic,
		GoodsSaleInfo:         temuProduct.GoodsSaleInfo,
		GoodsServicePromise:   temuProduct.GoodsServicePromise,
		GoodsExtensionInfo:    temuProduct.GoodsExtensionInfo,
		Extra:                 extra,
		CanSave:               true,
		SupportMaxRetailPrice: true,
		PlatformExpressBill:   false,
		SkcList:               temuProduct.SkcList,
	}

	h.logger.WithFields(logrus.Fields{
		"skc_count": len(request.SkcList),
		"sku_count": h.utils.GetTotalSkuCount(request.SkcList),
	}).Info("构建提交请求完成")

	return request
}

// validateSpecCompleteness 验证规格完整性
func (h *ProductSubmitHandler) validateSpecCompleteness(temuCtx *temucontext.TemuTaskContext) error {
	specDimensions, err := h.validator.ValidateSpecCompleteness(temuCtx)
	if err != nil {
		// 尝试自动修复规格配置
		if fixErr := h.fixer.AutoFixSpecConfiguration(temuCtx, specDimensions); fixErr != nil {
			h.logger.WithError(fixErr).Error("自动修复规格配置失败")
			return err
		}
		h.logger.Info("已自动修复规格配置")
	}
	return nil
}

// validateMultiplePackageConfiguration 验证多件套包装配置
func (h *ProductSubmitHandler) validateMultiplePackageConfiguration(temuCtx *temucontext.TemuTaskContext) error {
	hasInvalidPackaging, multipleSetsPropIndex, err := h.validator.ValidateMultiplePackageConfiguration(temuCtx)
	if err != nil {
		return err
	}

	// 如果包装配置不正确，修复它
	if hasInvalidPackaging {
		return h.fixer.FixMultiplePackageConfiguration(temuCtx, multipleSetsPropIndex)
	}

	return nil
}
