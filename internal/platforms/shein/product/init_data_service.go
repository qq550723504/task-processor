package product

import (
	product "task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"
)

// InitProductDataHandler 初始化产品数据处理器
type InitProductDataHandler struct {
}

// NewInitProductDataHandler 创建新的初始化产品数据处理器
func NewInitProductDataHandler() *InitProductDataHandler {
	return &InitProductDataHandler{}
}

// Name 返回处理器名称
func (h *InitProductDataHandler) Name() string {
	return "初始化产品数据"
}

// Handle 执行初始化产品数据处理
func (h *InitProductDataHandler) Handle(ctx *model.TaskContext) error {

	// 初始化ProductData字段
	ctx.ProductData = &product.Product{}

	return nil
}
