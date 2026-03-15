package build

import (
	"fmt"
	"task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/product"

	"github.com/google/uuid"
)

// BuildSpuHandler 构建最终发品数据处理器
type BuildSpuHandler struct {
}

// NewBuildSpuHandler 创建新的构建最终发品数据处理器
func NewBuildSpuHandler() *BuildSpuHandler {
	return &BuildSpuHandler{}
}

// Name 返回处理器名称
func (h *BuildSpuHandler) Name() string {
	return "构建最终的发品数据"
}

// Handle 执行构建最终发品数据处理
func (h *BuildSpuHandler) Handle(ctx *model.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		return fmt.Errorf("产品数据未获取，请先执行获取产品数据步骤")
	}

	if err := buildSpuData(ctx); err != nil {
		return fmt.Errorf("构建最终发品数据失败: %w", err)
	}

	return nil
}

func buildSpuData(ctx *model.TaskContext) error {
	// 构建最终发品数据
	SupplierCode := product.GetSkuByAsin(ctx, ctx.Task.ProductID)
	ctx.ProductData.SupplierCode = SupplierCode
	ctx.ProductData.PointKey = uuid.New().String()

	return nil
}

