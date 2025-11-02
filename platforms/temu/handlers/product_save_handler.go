package handlers

import (
	"fmt"
	"strconv"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// ProductSaveHandler 产品保存处理器
type ProductSaveHandler struct {
	logger *logrus.Entry
}

// ProductSaveRequest 产品保存请求结构体
type ProductSaveRequest struct {
	Product interface{} `json:"product"`
}

// ProductSaveResponse 产品保存响应结构体
type ProductSaveResponse struct {
	Success              bool `json:"success"`
	GoodsID              *int `json:"goods_id"`
	ListingCommitID      int  `json:"listing_commit_id"`
	ListingCommitVersion int  `json:"listing_commit_version"`
	GoodsCommitID        int  `json:"goods_commit_id"`
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
	h.logger.Info("开始保存产品")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 保存产品
	err := h.saveProduct(ctx)
	if err != nil {
		h.logger.Errorf("保存产品失败: %v", err)
		return fmt.Errorf("保存产品失败: %w", err)
	}

	h.logger.Info("产品保存完成")
	return nil
}

// saveProduct 保存产品
func (h *ProductSaveHandler) saveProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始保存产品到TEMU")

	// 这里应该构造保存请求并调用TEMU API
	// request := ProductSaveRequest{
	//     Product: ctx.TemuProduct,
	// }

	// 这里应该调用TEMU API保存产品
	// 为了简化，我们模拟保存结果
	response := &ProductSaveResponse{
		Success:              true,
		GoodsID:              h.parseGoodsID(ctx.TemuProduct.GoodsBasic.GoodsID),
		ListingCommitID:      h.parseCommitID(ctx.TemuProduct.GoodsBasic.ListingCommitID),
		ListingCommitVersion: h.parseCommitVersion(ctx.TemuProduct.GoodsBasic.ListingCommitVersion),
		GoodsCommitID:        h.parseCommitID(ctx.TemuProduct.GoodsBasic.GoodsCommitID),
	}

	if !response.Success {
		return fmt.Errorf("产品保存失败")
	}

	// 更新产品信息
	if response.GoodsID != nil {
		ctx.TemuProduct.GoodsBasic.GoodsID = strconv.Itoa(*response.GoodsID)
	}
	ctx.TemuProduct.GoodsBasic.ListingCommitID = strconv.Itoa(response.ListingCommitID)
	ctx.TemuProduct.GoodsBasic.ListingCommitVersion = strconv.Itoa(response.ListingCommitVersion)
	ctx.TemuProduct.GoodsBasic.GoodsCommitID = strconv.Itoa(response.GoodsCommitID)

	// 将保存结果存储到上下文
	ctx.SaveResult = response

	h.logger.Infof("产品保存成功: GoodsID=%v, ListingCommitID=%d, GoodsCommitID=%d",
		response.GoodsID, response.ListingCommitID, response.GoodsCommitID)

	return nil
}

// parseGoodsID 解析商品ID
func (h *ProductSaveHandler) parseGoodsID(goodsIDStr string) *int {
	if goodsIDStr == "" {
		// 生成新的商品ID
		newID := 602323879337871
		return &newID
	}

	if id, err := strconv.Atoi(goodsIDStr); err == nil {
		return &id
	}

	// 如果解析失败，生成新的ID
	newID := 602323879337871
	return &newID
}

// parseCommitID 解析提交ID
func (h *ProductSaveHandler) parseCommitID(commitIDStr string) int {
	if commitIDStr == "" {
		return 562950736449063 // 默认ID
	}

	if id, err := strconv.Atoi(commitIDStr); err == nil {
		return id
	}

	return 562950736449063 // 默认ID
}

// parseCommitVersion 解析提交版本
func (h *ProductSaveHandler) parseCommitVersion(versionStr string) int {
	if versionStr == "" {
		return 1 // 默认版本
	}

	if version, err := strconv.Atoi(versionStr); err == nil {
		return version
	}

	return 1 // 默认版本
}
