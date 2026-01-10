package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/pipeline"
	management_api "task-processor/internal/pkg/management/api"
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

// ProductSubmitRequest TEMU产品提交请求结构体（完整版）
type ProductSubmitRequest struct {
	GoodsBasic            types.GoodsBasic          `json:"goods_basic"`
	GoodsSaleInfo         types.GoodsSaleInfo       `json:"goods_sale_info"`
	GoodsServicePromise   types.GoodsServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    types.GoodsExtensionInfo  `json:"goods_extension_info"`
	Extra                 types.Extra               `json:"extra"`
	CanSave               bool                      `json:"can_save"`
	SupportMaxRetailPrice bool                      `json:"support_max_retail_price"`
	PlatformExpressBill   bool                      `json:"platform_express_bill"`
	SkcList               []types.Skc               `json:"skc_list"`
	//BatchSkuInfo          types.BatchSkuInfo        `json:"batch_sku_info"`
}

// ProductSubmitResponse TEMU产品提交响应结构体
type ProductSubmitResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Message   string              `json:"error_msg"`
	Result    ProductSubmitResult `json:"result"`
}

// ProductSubmitResult 产品提交结果
type ProductSubmitResult struct {
	SubmitSuccess           bool `json:"submit_success"`
	EditCustomizedInfoAlert bool `json:"edit_customized_info_alert"`
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

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/submit",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": request,
	}

	// 发送请求到TEMU API
	response := &ProductSubmitResponse{}
	err := temuCtx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		// 保存JSON数据到文件用于调试
		task := temuCtx.GetTask()
		if task != nil {
			taskID := fmt.Sprintf("%d", task.ID)
			if saveErr := h.utils.SaveJSONToFile(taskID, requestJSON, "product_submit"); saveErr != nil {
				h.logger.Errorf("保存JSON文件失败: %v", saveErr)
			}
		}
		h.logger.Errorf("发送TEMU API请求失败: %v", err)
		return fmt.Errorf("发送提交请求失败: %w", err)
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

	if !response.Result.SubmitSuccess {
		h.logger.Errorf("产品提交结果失败: submit_success=%v", response.Result.SubmitSuccess)
		responseJSON, _ := h.utils.MarshalWithoutHTMLEscape(response)
		h.logger.Errorf("完整响应: %s", string(responseJSON))

		// 提交结果失败时保存到草稿箱
		h.logger.Warnf("产品提交结果失败，尝试保存到草稿箱...")
		if saveErr := h.saveHandler.Handle(temuCtx); saveErr != nil {
			h.logger.Errorf("保存到草稿箱也失败: %v", saveErr)
			h.logger.Error("❌ 提交和保存草稿都失败，任务将被标记为不可重试")
			// 提交失败且保存草稿也失败，标记为不可重试
			return fmt.Errorf("NONRETRYABLE: 产品提交未成功且保存草稿失败: %w", saveErr)
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
func (h *ProductSubmitHandler) buildSubmitRequest(temuCtx *temucontext.TemuTaskContext) *ProductSubmitRequest {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct

	// 转换Extra类型
	extra := types.Extra{
		Tab:              temuProduct.Extra.Tab,
		MinSkuImageSize:  temuProduct.Extra.MinSkuImageSize,
		MaxSkuImageSize:  temuProduct.Extra.MaxSkuImageSize,
		CreateEmptyGoods: temuProduct.Extra.CreateEmptyGoods,
	}

	request := &ProductSubmitRequest{
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
