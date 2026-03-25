package store

import shein "task-processor/internal/shein"

type WarehouseInfoHandler struct{}

func NewWarehouseInfoHandler() *WarehouseInfoHandler {
	return &WarehouseInfoHandler{}
}

func (h *WarehouseInfoHandler) Name() string {
	return "warehouse_info"
}

func (h *WarehouseInfoHandler) Handle(ctx *shein.TaskContext) error {
	warehouseInfo, err := ctx.WarehouseAPI.GetWarehouses()
	if err != nil {
		return shein.NewRetryableError("get warehouses failed", err)
	}

	ctx.SetWarehouses(warehouseInfo)
	return nil
}
