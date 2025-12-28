package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// ProductSaveHandler 产品保存处理器
type ProductSaveHandler struct {
	logger    *logrus.Entry
	fileUtils *utils.FileUtils
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
		logger:    logrus.WithField("handler", "ProductSaveHandler"),
		fileUtils: utils.NewFileUtils(),
	}
}

// Name 返回处理器名称
func (h *ProductSaveHandler) Name() string {
	return "产品保存处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *ProductSaveHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始保存产品到草稿箱")

	// 检查任务上下文中的必要数据
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 保存产品
	err := h.saveProduct(temuCtx)
	if err != nil {
		h.logger.Errorf("保存产品失败: %v", err)
		return fmt.Errorf("保存产品失败: %w", err)
	}

	h.logger.Info("产品保存完成")
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *ProductSaveHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// saveProduct 保存产品到草稿箱
func (h *ProductSaveHandler) saveProduct(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始保存产品到TEMU草稿箱")

	// 获取API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 构造TEMU产品保存请求
	request := h.buildSaveRequest(temuCtx)

	// 移除调试文件输出，仅保留app.log

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

	err := temuCtx.APIClient.SendTEMURequest(apiReq, response)
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
		h.updateProductWithSaveResult(temuCtx, response.Result)
	} else {
		h.logger.Info("产品保存成功，但未返回详细结果")
	}

	// 将保存结果存储到强类型字段
	temuCtx.SaveResult = response

	return nil
}

// buildSaveRequest 构建保存请求
func (h *ProductSaveHandler) buildSaveRequest(temuCtx *temucontext.TemuTaskContext) *ProductSaveRequest {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct

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
func (h *ProductSaveHandler) updateProductWithSaveResult(temuCtx *temucontext.TemuTaskContext, result *ProductSaveResult) {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct
	if temuProduct == nil {
		h.logger.Error("TEMU产品信息为空")
		return
	}

	basic := &temuProduct.GoodsBasic

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

	h.logger.Info("产品信息已更新")
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
