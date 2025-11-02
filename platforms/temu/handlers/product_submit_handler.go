package handlers

import (
	"encoding/json"
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// ProductSubmitHandler 产品提交处理器
type ProductSubmitHandler struct {
	logger *logrus.Entry
}

// ProductSubmitRequest 产品提交请求结构体
type ProductSubmitRequest struct {
	Product interface{} `json:"product"`
}

// ProductSubmitResponse 产品提交响应结构体
type ProductSubmitResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// NewProductSubmitHandler 创建新的产品提交处理器
func NewProductSubmitHandler() *ProductSubmitHandler {
	return &ProductSubmitHandler{
		logger: logrus.WithField("handler", "ProductSubmitHandler"),
	}
}

// Name 返回处理器名称
func (h *ProductSubmitHandler) Name() string {
	return "产品提交处理器"
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

	// 验证产品数据
	err := h.validateProduct(ctx)
	if err != nil {
		h.logger.Errorf("产品数据验证失败: %v", err)
		return fmt.Errorf("产品数据验证失败: %w", err)
	}

	// 提交产品
	err = h.submitProduct(ctx)
	if err != nil {
		h.logger.Errorf("提交产品失败: %v", err)
		return fmt.Errorf("提交产品失败: %w", err)
	}

	h.logger.Info("产品提交完成")
	return nil
}

// validateProduct 验证产品数据
func (h *ProductSubmitHandler) validateProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("验证产品数据")

	product := ctx.TemuProduct

	// 验证基本信息
	if product.GoodsBasic.GoodsName == "" {
		return fmt.Errorf("商品名称不能为空")
	}

	if product.GoodsBasic.CatID == 0 {
		return fmt.Errorf("分类ID不能为空")
	}

	if product.GoodsBasic.HdThumbURL == "" {
		return fmt.Errorf("主图不能为空")
	}

	// 验证SKC和SKU
	if len(product.SkcList) == 0 {
		return fmt.Errorf("SKC列表不能为空")
	}

	for i, skc := range product.SkcList {
		if skc.SkcID == "" {
			return fmt.Errorf("SKC[%d] ID不能为空", i+1)
		}

		if len(skc.SkuList) == 0 {
			return fmt.Errorf("SKC[%d] SKU列表不能为空", i+1)
		}

		for j, sku := range skc.SkuList {
			if sku.SkuID == "" {
				return fmt.Errorf("SKU[%d-%d] ID不能为空", i+1, j+1)
			}

			if sku.Price <= 0 {
				return fmt.Errorf("SKU[%d-%d] 价格必须大于0", i+1, j+1)
			}

			if sku.Quantity < 0 {
				return fmt.Errorf("SKU[%d-%d] 库存不能为负数", i+1, j+1)
			}
		}
	}

	// 验证服务承诺
	if product.GoodsServicePromise.ShipmentLimitSecond <= 0 {
		return fmt.Errorf("发货时限必须大于0")
	}

	h.logger.Info("产品数据验证通过")
	return nil
}

// submitProduct 提交产品
func (h *ProductSubmitHandler) submitProduct(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始提交产品到TEMU")

	// 这里应该构造提交请求并调用TEMU API
	// request := ProductSubmitRequest{
	//     Product: ctx.TemuProduct,
	// }

	// 记录提交的产品信息（用于调试）
	productJSON, err := json.Marshal(ctx.TemuProduct)
	if err != nil {
		h.logger.Errorf("序列化产品信息失败: %v", err)
	} else {
		h.logger.Debugf("提交的产品信息: %s", string(productJSON))
	}

	// 这里应该调用TEMU API提交产品
	// 为了简化，我们模拟提交结果
	response := &ProductSubmitResponse{
		Success:   true,
		Message:   "产品提交成功",
		RequestID: h.generateRequestID(),
	}

	if !response.Success {
		return fmt.Errorf("产品提交失败: %s", response.Message)
	}

	// 记录提交结果
	h.logger.Infof("产品提交成功: RequestID=%s, Message=%s",
		response.RequestID, response.Message)

	// 将提交结果存储到上下文
	ctx.SubmitResult = response

	return nil
}

// generateRequestID 生成请求ID
func (h *ProductSubmitHandler) generateRequestID() string {
	// 这里应该生成真实的请求ID
	return fmt.Sprintf("req_%d", 1700000000000+12345)
}
