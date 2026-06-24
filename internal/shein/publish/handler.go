// Package publish 提供SHEIN平台产品发布核心处理器
package publish

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/listingruntime"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein"
	product "task-processor/internal/shein/api/product"
)

// PublishProductHandler 发布产品处理器
type PublishProductHandler struct {
	validator     *PublishProductValidator
	errorHandler  *PublishProductErrorHandler
	checker       *PublishProductChecker
	debugSaveJSON bool
}

// NewPublishProductHandler 创建新的发布产品处理器。
// debugSaveJSON 为 true 时，发布前将产品数据保存为 JSON 文件（仅用于调试）。
func NewPublishProductHandler(debugSaveJSON bool) *PublishProductHandler {
	return &PublishProductHandler{
		validator:     NewPublishProductValidator(),
		errorHandler:  NewPublishProductErrorHandler(),
		checker:       NewPublishProductChecker(),
		debugSaveJSON: debugSaveJSON,
	}
}

// Name 返回处理器名称
func (h *PublishProductHandler) Name() string {
	return "发布产品"
}

// Handle 执行发布产品处理
func (h *PublishProductHandler) Handle(ctx *shein.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return shein.NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	// 方案3：发布前预验证
	logger.GetGlobalLogger("shein/publish").Info("🔍 开始发布前预验证...")

	validationInput, err := buildValidationInput(ctx)
	if err != nil {
		return shein.NewNonRetryableError("构建发布验证输入失败", err)
	}

	if err := h.validator.PreValidateProductData(ctx, validationInput); err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("❌ 发布前预验证失败: %v", err)
		h.maybeDebugSave(ctx, "validation_failed")
		// 预验证失败通常是数据问题，可重试（可能通过重新处理解决）
		return shein.NewRetryableError("发布前预验证失败", err)
	}

	logger.GetGlobalLogger("shein/publish").Info("✅ 发布前预验证通过")
	h.maybeDebugSave(ctx, "")

	// 店铺开启草稿模式时，正常链路改为保存到草稿箱而不是直接发布。
	response, err := h.submitProduct(ctx)
	if err != nil {
		// 发布失败可能是网络问题或临时性错误，可重试
		return shein.NewRetryableError("发布产品失败", err)
	}

	return h.errorHandler.HandlePublishResponse(ctx, response)
}

func (h *PublishProductHandler) submitProduct(ctx *shein.TaskContext) (*product.SheinResponse, error) {
	if shouldSaveDraftByStore(ctx) {
		return h.SaveDraftProduct(ctx)
	}
	return h.publishProduct(ctx)
}

// publishProduct 统一的产品发布方法
func (h *PublishProductHandler) publishProduct(ctx *shein.TaskContext) (*product.SheinResponse, error) {
	input, err := buildPublishProductInput(ctx)
	if err != nil {
		return nil, err
	}
	return doPublishProduct(ctx, input)
}

// doPublishProduct 包级发布函数，供 error_handler 等复用，避免零值实例化 PublishProductHandler。
func doPublishProduct(ctx *shein.TaskContext, input *PublishProductInput) (*product.SheinResponse, error) {
	response, _, err := input.ProductAPI.PublishProduct(input.ProductData)
	ctx.SetSheinResponse(response)
	return response, err
}

// SaveDraftProduct 保存产品到草稿箱
func (h *PublishProductHandler) SaveDraftProduct(ctx *shein.TaskContext) (*product.SheinResponse, error) {
	input, err := buildPublishProductInput(ctx)
	if err != nil {
		return nil, err
	}

	response, _, err := input.ProductAPI.SaveDraftProduct(input.ProductData)
	ctx.SetSheinResponse(response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func shouldSaveDraftByStore(ctx *shein.TaskContext) bool {
	if ctx == nil {
		return false
	}
	return storeDraftEnabled(publishRuntimeStoreInfo(ctx.StoreInfo))
}

func storeDraftEnabled(storeInfo *listingruntime.StoreInfo) bool {
	return storeInfo != nil && storeInfo.EnableDraft != nil && *storeInfo.EnableDraft
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *PublishProductHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalWithoutHTMLEscape(v)
}

// maybeDebugSave 仅在 debugSaveJSON 开启时将产品数据保存为 JSON 文件。
// suffix 非空时追加到文件名中（如 "validation_failed"）。
func (h *PublishProductHandler) maybeDebugSave(ctx *shein.TaskContext, suffix string) {
	if !h.debugSaveJSON || ctx.Task == nil || ctx.ProductData == nil {
		return
	}
	taskID := fmt.Sprintf("%d", ctx.Task.ID)
	var filename string
	if suffix != "" {
		filename = fmt.Sprintf("%s_%s_%s.json", ctx.Task.ProductID, taskID, suffix)
	} else {
		filename = fmt.Sprintf("%s_%s.json", ctx.Task.ProductID, taskID)
	}
	jsonData, err := jsonx.MarshalWithoutHTMLEscape(ctx.ProductData)
	if err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("序列化产品数据失败: %v", err)
		return
	}
	if err := jsonx.SaveToFile(filename, jsonData); err != nil {
		logger.GetGlobalLogger("shein/publish").Errorf("保存调试JSON文件失败: %v", err)
		return
	}
	logger.GetGlobalLogger("shein/publish").Infof("📄 调试JSON已保存: logs/%s", filename)
}
