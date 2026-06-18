package product

import (
	"strings"

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
	productData := &productapi.Product{}
	if ctx != nil && ctx.AuthorizedBrand != nil {
		if code := strings.TrimSpace(ctx.AuthorizedBrand.Code); code != "" {
			productData.BrandCode = &code
		}
	}
	ctx.SetProductData(productData)
	return nil
}
