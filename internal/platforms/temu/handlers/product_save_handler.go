package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ProductSaveHandler 产品保存处理器
type ProductSaveHandler struct {
	logger *logrus.Entry
}

// ProductSaveRequest TEMU产品保存请求结构体
type ProductSaveRequest struct {
	GoodsBasic            types.GoodsBasic          `json:"goods_basic"`
	GoodsSaleInfo         types.GoodsSaleInfo       `json:"goods_sale_info"`
	GoodsServicePromise   types.GoodsServicePromise `json:"goods_service_promise"`
	GoodsExtensionInfo    types.GoodsExtensionInfo  `json:"goods_extension_info"`
	Extra                 types.Extra               `json:"extra"`
	CanSave               bool                      `json:"can_save"`
	SupportMaxRetailPrice bool                      `json:"support_max_retail_price"`
	PlatformExpressBill   bool                      `json:"platform_express_bill"`
	SkcList               []types.Skc               `json:"skc_list"`
	BatchSkuInfo          types.BatchSkuInfo        `json:"batch_sku_info"`
}

// ProductSaveResponse TEMU产品保存响应结构体
type ProductSaveResponse struct {
	Success   bool               `json:"success"`
	ErrorCode int                `json:"error_code"`
	Message   string             `json:"error_msg,omitempty"`
	Result    *ProductSaveResult `json:"result,omitempty"`
}

// ProductSaveResult 产品保存结果
type ProductSaveResult struct {
	ListingCommitID      string `json:"listing_commit_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	GoodsCommitID        string `json:"goods_commit_id"`
}

// NewProductSaveHandler 创建新的产品保存处理器
func NewProductSaveHandler() *ProductSaveHandler {
	return &ProductSaveHandler{
		logger: logrus.WithField("handler", "ProductSaveHandler"),
	}
}

// Name 返回处理器名称
func (h *ProductSaveHandler) Name() string {
	return "产品保存处理器"
}

// Handle 处理任务
func (h *ProductSaveHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始保存产品到草稿箱")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 保存产品到草稿箱
	err := h.saveProduct(ctx)
	if err != nil {
		h.logger.Errorf("保存产品失败: %v", err)
		return fmt.Errorf("保存产品失败: %w", err)
	}

	h.logger.Info("产品保存到草稿箱完成")
	return nil
}

// saveProduct 保存产品到草稿箱
func (h *ProductSaveHandler) saveProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始保存产品到TEMU草稿箱")

	// 检查API客户端
	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 构造TEMU产品保存请求
	request := h.buildSaveRequest(ctx)

	// 记录保存的产品信息（用于调试）
	requestJSON, err := h.marshalWithoutHTMLEscape(request)
	if err != nil {
		h.logger.Errorf("序列化保存请求失败: %v", err)
	} else {
		h.logger.Infof("=== 保存到草稿箱的JSON数据 ===")
		h.logger.Infof("%s", string(requestJSON))
		h.logger.Infof("=== JSON数据结束 ===")
	}

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/save",
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
	response := &ProductSaveResponse{}
	err = ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("发送TEMU API请求失败: %v", err)
		h.logger.Errorf("请求URL: %s", apiReq["url"])
		h.logger.Errorf("请求方法: %s", apiReq["method"])
		return fmt.Errorf("发送保存请求失败: %w", err)
	}

	// 检查响应结果
	if !response.Success {
		h.logger.Errorf("TEMU API响应失败: success=%v, error_code=%d", response.Success, response.ErrorCode)
		if response.Message != "" {
			h.logger.Errorf("错误信息: %s", response.Message)
		}
		responseJSON, _ := h.marshalWithoutHTMLEscape(response)
		h.logger.Errorf("完整响应: %s", string(responseJSON))

		return fmt.Errorf("产品保存失败: error_code=%d, message=%s", response.ErrorCode, response.Message)
	}

	// 记录成功的响应信息
	h.logger.Infof("TEMU API响应成功: success=%v, error_code=%d", response.Success, response.ErrorCode)

	// 记录保存结果
	if response.Result != nil {
		// 更新产品信息中的ID
		h.updateProductWithSaveResult(ctx, response.Result)
	} else {
		h.logger.Info("产品保存成功，但未返回详细结果")
	}

	// 将保存结果存储到上下文
	ctx.SetData("save_response", response)

	return nil
}

// buildSaveRequest 构建保存请求
func (h *ProductSaveHandler) buildSaveRequest(ctx *pipeline.TaskContext) *ProductSaveRequest {
	temuProduct := ctx.TemuProduct

	// 转换Extra类型
	extra := types.Extra{
		Tab:              temuProduct.Extra.Tab,
		MinSkuImageSize:  temuProduct.Extra.MinSkuImageSize,
		MaxSkuImageSize:  temuProduct.Extra.MaxSkuImageSize,
		CreateEmptyGoods: temuProduct.Extra.CreateEmptyGoods,
	}

	request := &ProductSaveRequest{
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

	h.logger.Infof("构建保存请求完成: SKC数量=%d, 总SKU数量=%d",
		len(request.SkcList), h.getTotalSkuCount(request.SkcList))

	// 打印关键字段信息用于调试
	h.logger.Infof("商品基本信息: ID=%s, 名称=%s", request.GoodsBasic.GoodsID, request.GoodsBasic.GoodsName)
	h.logger.Infof("分类信息: CatID=%d", request.GoodsBasic.CatID)
	h.logger.Infof("请求标志: CanSave=%v, SupportMaxRetailPrice=%v, PlatformExpressBill=%v",
		request.CanSave, request.SupportMaxRetailPrice, request.PlatformExpressBill)

	return request
}

// updateProductWithSaveResult 使用保存结果更新产品信息
func (h *ProductSaveHandler) updateProductWithSaveResult(ctx *pipeline.TaskContext, result *ProductSaveResult) {
	basic := &ctx.TemuProduct.GoodsBasic

	// 更新产品ID信息
	if result.ListingCommitID != "" {
		basic.ListingCommitID = result.ListingCommitID
		h.logger.Infof("更新ListingCommitID: %s", basic.ListingCommitID)
	}

	if result.ListingCommitVersion != "" {
		basic.ListingCommitVersion = result.ListingCommitVersion
		h.logger.Infof("更新ListingCommitVersion: %s", basic.ListingCommitVersion)
	}

	if result.GoodsCommitID != "" {
		basic.GoodsCommitID = result.GoodsCommitID
		h.logger.Infof("更新GoodsCommitID: %s", basic.GoodsCommitID)
	}
}

// getTotalSkuCount 获取总SKU数量
func (h *ProductSaveHandler) getTotalSkuCount(skcList []types.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *ProductSaveHandler) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
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
