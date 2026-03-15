package store

import "task-processor/internal/platforms/shein"

// WarehouseInfoHandler 获取仓库信息处理器
type WarehouseInfoHandler struct {
}

// NewWarehouseInfoHandler 创建新的仓库信息处理器
func NewWarehouseInfoHandler() *WarehouseInfoHandler {
	return &WarehouseInfoHandler{}
}

// Name 返回步骤名称
func (h *WarehouseInfoHandler) Name() string {
	return "获取仓库信息"
}

// Handle 执行步骤处理
func (h *WarehouseInfoHandler) Handle(ctx *shein.TaskContext) error {
	// 调用API获取仓库信息
	warehouseInfo, err := ctx.WarehouseAPI.GetWarehouses()
	if err != nil {
		// 网络请求失败可重试
		return shein.NewRetryableError("获取仓库信息失败", err)
	}

	// 将仓库信息存储到上下文的专门字段中
	ctx.Warehouses = warehouseInfo

	return nil
}


