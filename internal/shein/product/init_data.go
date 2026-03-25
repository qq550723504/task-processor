package product

import (
	shein "task-processor/internal/shein"
	productapi "task-processor/internal/shein/api/product"
)

type InitProductDataHandler struct{}

func NewInitProductDataHandler() *InitProductDataHandler {
	return &InitProductDataHandler{}
}

func (h *InitProductDataHandler) Name() string {
	return "init_product_data"
}

func (h *InitProductDataHandler) Handle(ctx *shein.TaskContext) error {
	ctx.SetProductData(&productapi.Product{})
	return nil
}
