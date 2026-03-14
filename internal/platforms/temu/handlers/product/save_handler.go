package product

import (
	"fmt"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/fileutil"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ProductSaveHandler 产品保存处理器
type ProductSaveHandler struct {
	logger    *logrus.Entry
	fileUtils *fileutil.FileUtil
}

// NewProductSaveHandler 创建新的产品保存处理器
func NewProductSaveHandler() *ProductSaveHandler {
	return &ProductSaveHandler{
		logger:    logrus.WithField("handler", "ProductSaveHandler"),
		fileUtils: fileutil.New(),
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

	// 创建ProductAPI
	productAPI := api.NewProductAPI(temuCtx.APIClient, h.logger)

	// 调用API保存产品
	response, err := productAPI.SaveProduct(request)
	if err != nil {
		h.logger.Errorf("保存产品失败: %v", err)
		return fmt.Errorf("保存产品失败: %w", err)
	}

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
func (h *ProductSaveHandler) buildSaveRequest(temuCtx *temucontext.TemuTaskContext) *models.ProductSaveRequest {
	// 获取TEMU产品信息
	temuProduct := temuCtx.TemuProduct

	// 转换Extra类型
	extra := models.Extra{
		Tab:              temuProduct.Extra.Tab,
		MinSkuImageSize:  temuProduct.Extra.MinSkuImageSize,
		MaxSkuImageSize:  temuProduct.Extra.MaxSkuImageSize,
		CreateEmptyGoods: temuProduct.Extra.CreateEmptyGoods,
	}

	request := &models.ProductSaveRequest{
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
func (h *ProductSaveHandler) updateProductWithSaveResult(temuCtx *temucontext.TemuTaskContext, result *models.ProductSaveResult2) {
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
func (h *ProductSaveHandler) getTotalSkuCount(skcList []models.Skc) int {
	total := 0
	for _, skc := range skcList {
		total += len(skc.SkuList)
	}
	return total
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *ProductSaveHandler) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	return jsonutil.MarshalWithoutHTMLEscape(v)
}
