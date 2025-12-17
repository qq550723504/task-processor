package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	management_api "task-processor/internal/common/management/api"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ProductSubmitHandler 产品提交处理器
type ProductSubmitHandler struct {
	logger        *logrus.Entry
	saveHandler   *ProductSaveHandler
	mappingClient management_api.ProductImportMappingAPI
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
	return &ProductSubmitHandler{
		logger:        logrus.WithField("handler", "ProductSubmitHandler"),
		saveHandler:   NewProductSaveHandler(),
		mappingClient: mappingClient,
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

// Handle 处理任务
func (h *ProductSubmitHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始提交产品")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 提交产品（产品存在性检查已在第3步完成）
	err := h.submitProduct(ctx)
	if err != nil {
		h.logger.Errorf("提交产品失败: %v", err)
		return fmt.Errorf("提交产品失败: %w", err)
	}

	h.logger.Info("产品提交完成")
	return nil
}

// submitProduct 提交产品
func (h *ProductSubmitHandler) submitProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始提交产品到TEMU")

	// 检查API客户端
	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 【最后的格式检查】在提交前确保产品名称格式正确
	if ctx.TemuProduct != nil && ctx.TemuProduct.GoodsBasic.GoodsName != "" {
		originalName := ctx.TemuProduct.GoodsBasic.GoodsName
		fixedName := h.ensureProperFormatting(originalName)

		if fixedName != originalName {
			h.logger.Warnf("⚠️ 提交前修复产品名称格式: '%s' -> '%s'", originalName, fixedName)
			ctx.TemuProduct.GoodsBasic.GoodsName = fixedName
		}
	}

	// 构造TEMU产品提交请求
	request := h.buildSubmitRequest(ctx)

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
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("发送TEMU API请求失败: %v", err)
		h.logger.Errorf("请求URL: %s", apiReq["url"])
		h.logger.Errorf("请求方法: %s", apiReq["method"])
		return fmt.Errorf("发送提交请求失败: %w", err)
	}

	h.logger.Infof("out_goods_sn: %s", request.GoodsBasic.OutGoodsSN)

	// 检查响应结果
	if !response.Success {
		h.logger.Errorf("TEMU API响应失败: success=%v, error_code=%d, error_message=%v", response.Success, response.ErrorCode, response.Message)

		// 记录提交的产品信息（用于调试）
		requestJSON, err := h.marshalWithoutHTMLEscape(request)
		if err != nil {
			h.logger.Errorf("序列化提交请求失败: %v", err)
		} else {

			h.logger.Infof("=== 最终提交的JSON数据 ===")
			h.logger.Infof("%s", string(requestJSON))
			h.logger.Infof("=== JSON数据结束 ===")
		}

		// 检查是否为不可重试的错误
		if h.isNonRetryableError(response.ErrorCode, response.Message) {
			h.logger.Errorf("❌ 检测到不可重试错误(error_code=%d): %s", response.ErrorCode, response.Message)
			h.logger.Error("❌ 此错误无法通过重试解决，任+务将被标记为失败")
			// 返回特殊错误，让上层知道这是不可重试的
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d): %s", response.ErrorCode, response.Message)
		}

		// 可重试的错误，尝试保存到草稿箱
		h.logger.Warnf("产品提交失败，尝试保存到草稿箱...")
		if saveErr := h.saveHandler.Handle(ctx); saveErr != nil {
			h.logger.Errorf("保存到草稿箱也失败: %v", saveErr)
			h.logger.Error("❌ 提交和保存草稿都失败，任务将被标记为不可重试")
			// 提交失败且保存草稿也失败，标记为不可重试
			return fmt.Errorf("NONRETRYABLE: 产品提交失败(error_code=%d)且保存草稿失败: %w", response.ErrorCode, saveErr)
		}
		h.logger.Infof("✅ 产品已保存到草稿箱，任务标记为已完成")
		// 保存到草稿箱成功，标记为特殊的成功状态，避免重复处理
		ctx.SetData("saved_to_draft", true)
		return nil // 返回nil表示处理成功，不再重试
	}

	if !response.Result.SubmitSuccess {
		h.logger.Errorf("产品提交结果失败: submit_success=%v", response.Result.SubmitSuccess)
		responseJSON, _ := h.marshalWithoutHTMLEscape(response)
		h.logger.Errorf("完整响应: %s", string(responseJSON))

		// 提交结果失败时保存到草稿箱
		h.logger.Warnf("产品提交结果失败，尝试保存到草稿箱...")
		if saveErr := h.saveHandler.Handle(ctx); saveErr != nil {
			h.logger.Errorf("保存到草稿箱也失败: %v", saveErr)
			h.logger.Error("❌ 提交和保存草稿都失败，任务将被标记为不可重试")
			// 提交失败且保存草稿也失败，标记为不可重试
			return fmt.Errorf("NONRETRYABLE: 产品提交未成功且保存草稿失败: %w", saveErr)
		}
		h.logger.Infof("✅ 产品已保存到草稿箱，任务标记为已完成")
		// 保存到草稿箱成功，标记为特殊的成功状态，避免重复处理
		ctx.SetData("saved_to_draft", true)
		return nil // 返回nil表示处理成功，不再重试
	}

	// 保存提交响应和产品数据，供后续处理器使用
	ctx.SetData("submit_response", response)
	ctx.SetData("product_data", ctx.TemuProduct)

	h.logger.Infof("🎉 产品发布成功！商品ID: %s, 商品名称: %s", ctx.TemuProduct.GoodsBasic.GoodsID, ctx.TemuProduct.GoodsBasic.GoodsName)

	return nil
}

// buildSubmitRequest 构建提交请求
func (h *ProductSubmitHandler) buildSubmitRequest(ctx *pipeline.TaskContext) *ProductSubmitRequest {
	temuProduct := ctx.TemuProduct

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
		len(request.SkcList), h.getTotalSkuCount(request.SkcList))

	// 打印关键字段信息用于调试
	h.logger.Infof("商品基本信息: ID=%s, 名称=%s", request.GoodsBasic.GoodsID, request.GoodsBasic.GoodsName)
	h.logger.Infof("分类信息: CatID=%d", request.GoodsBasic.CatID)
	h.logger.Infof("请求标志: CanSave=%v, SupportMaxRetailPrice=%v, PlatformExpressBill=%v",
		request.CanSave, request.SupportMaxRetailPrice, request.PlatformExpressBill)

	return request
}

// getTotalSkuCount 获取总SKU数量
func (h *ProductSubmitHandler) getTotalSkuCount(skcList []types.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}

// isNonRetryableError 判断错误是否不可重试
func (h *ProductSubmitHandler) isNonRetryableError(errorCode int, errorMessage string) bool {
	// 定义不可重试的错误码
	nonRetryableErrorCodes := map[int]string{
		10000103: "SKU重复错误 - Contribution SKU duplicated",
		10000104: "商品已存在",
		10000105: "商品ID重复",
		// 可以根据实际情况添加更多不可重试的错误码
	}

	// 检查错误码
	if reason, exists := nonRetryableErrorCodes[errorCode]; exists {
		h.logger.Infof("识别到不可重试错误: %s (error_code=%d)", reason, errorCode)
		return true
	}

	// 检查错误消息中的关键词
	nonRetryableKeywords := []string{
		"duplicated",
		"duplicate",
		"already exists",
		"重复",
		"已存在",
	}

	for _, keyword := range nonRetryableKeywords {
		if strings.Contains(strings.ToLower(errorMessage), strings.ToLower(keyword)) {
			h.logger.Infof("错误消息包含不可重试关键词: %s", keyword)
			return true
		}
	}

	return false
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *ProductSubmitHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义，避免&被转义为\u0026

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}
