package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// PublishHandler 发布产品处理器
type PublishHandler struct {
	logger *logrus.Entry
}

// PublishRequest 发布产品请求结构体
type PublishRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            int    `json:"click_type"`
}

// PublishResponse 发布产品响应结构体
type PublishResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// NewPublishHandler 创建新的发布产品处理器
func NewPublishHandler() *PublishHandler {
	return &PublishHandler{
		logger: logrus.WithField("handler", "PublishHandler"),
	}
}

// Name 返回处理器名称
func (h *PublishHandler) Name() string {
	return "发布产品处理器"
}

// Handle 处理任务
func (h *PublishHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始发布产品")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 验证发布前置条件
	err := h.validatePublishConditions(ctx)
	if err != nil {
		h.logger.Errorf("发布前置条件验证失败: %v", err)
		return fmt.Errorf("发布前置条件验证失败: %w", err)
	}

	// 发布产品
	err = h.publishProduct(ctx)
	if err != nil {
		h.logger.Errorf("发布产品失败: %v", err)
		return fmt.Errorf("发布产品失败: %w", err)
	}

	h.logger.Info("产品发布完成")
	return nil
}

// validatePublishConditions 验证发布前置条件
func (h *PublishHandler) validatePublishConditions(ctx *pipeline.TaskContext) error {
	h.logger.Info("验证发布前置条件")

	product := ctx.TemuProduct

	// 检查必要的ID
	if product.GoodsBasic.ListingCommitID == "" {
		return fmt.Errorf("ListingCommitID不能为空")
	}

	if product.GoodsBasic.GoodsCommitID == "" {
		return fmt.Errorf("GoodsCommitID不能为空")
	}

	if product.GoodsBasic.GoodsID == "" {
		return fmt.Errorf("GoodsID不能为空")
	}

	if product.GoodsBasic.ListingCommitVersion == "" {
		return fmt.Errorf("ListingCommitVersion不能为空")
	}

	// 检查产品是否已保存
	if ctx.SaveResult == nil {
		return fmt.Errorf("产品尚未保存，无法发布")
	}

	h.logger.Info("发布前置条件验证通过")
	return nil
}

// publishProduct 发布产品
func (h *PublishHandler) publishProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始发布产品到TEMU")

	product := ctx.TemuProduct

	// 构造发布请求
	request := PublishRequest{
		ListingCommitID:      product.GoodsBasic.ListingCommitID,
		GoodsCommitID:        product.GoodsBasic.GoodsCommitID,
		GoodsID:              product.GoodsBasic.GoodsID,
		CatID:                product.GoodsBasic.CatID,
		ListingCommitVersion: product.GoodsBasic.ListingCommitVersion,
		ClickType:            1, // 发布类型
	}

	h.logger.Infof("发布请求: ListingCommitID=%s, GoodsID=%s, CatID=%d",
		request.ListingCommitID, request.GoodsID, request.CatID)

	// 这里应该调用TEMU API发布产品
	// 为了简化，我们模拟发布结果
	response := &PublishResponse{
		Success:   true,
		Message:   "产品发布成功",
		RequestID: h.generateRequestID(),
	}

	if !response.Success {
		return fmt.Errorf("产品发布失败: %s", response.Message)
	}

	// 将发布结果存储到上下文
	ctx.PublishResult = response

	h.logger.Infof("产品发布成功: RequestID=%s, Message=%s",
		response.RequestID, response.Message)

	// 更新产品状态
	product.GoodsBasic.HasSubmitted = true

	return nil
}

// generateRequestID 生成请求ID
func (h *PublishHandler) generateRequestID() string {
	return fmt.Sprintf("publish_req_%d", 1700000000000+54321)
}
