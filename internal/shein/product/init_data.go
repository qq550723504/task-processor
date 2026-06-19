package product

import (
	"context"
	"strings"

	shein "task-processor/internal/shein"
	productapi "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/authorizedbrand"
)

type authorizedBrandResolver interface {
	ResolveForProductBrand(ctx context.Context, cfg authorizedbrand.Config, productBrand string) (*authorizedbrand.Resolved, error)
}

type InitProductDataHandler struct{}

func NewInitProductDataHandler() *InitProductDataHandler {
	return &InitProductDataHandler{}
}

func (h *InitProductDataHandler) Name() string {
	return "init_product_data"
}

func (h *InitProductDataHandler) Handle(ctx *shein.TaskContext) error {
	productData := &productapi.Product{}
	if err := ensureAuthorizedBrandResolved(ctx); err != nil {
		return err
	}
	if ctx != nil && ctx.AuthorizedBrand != nil {
		if code := strings.TrimSpace(ctx.AuthorizedBrand.Code); code != "" {
			productData.BrandCode = &code
		}
	}
	ctx.SetProductData(productData)
	return nil
}

func ensureAuthorizedBrandResolved(ctx *shein.TaskContext) error {
	if ctx == nil || ctx.AuthorizedBrand != nil {
		return nil
	}

	storeCfg := authorizedbrand.ConfigFromStore(ctx.StoreInfo)
	if !storeCfg.Enabled || ctx.ProductAPI == nil {
		return nil
	}

	return ensureAuthorizedBrandResolvedWithResolver(ctx, storeCfg, authorizedbrand.NewResolver(ctx.ProductAPI))
}

func ensureAuthorizedBrandResolvedWithResolver(ctx *shein.TaskContext, storeCfg authorizedbrand.Config, resolver authorizedBrandResolver) error {
	if ctx == nil || resolver == nil {
		return nil
	}

	productBrand := ""
	if ctx.AmazonProduct != nil {
		productBrand = ctx.AmazonProduct.Brand
	}

	resolved, err := resolver.ResolveForProductBrand(ctx.Context, storeCfg, productBrand)
	if err != nil {
		return err
	}
	ctx.SetAuthorizedBrand(resolved)
	ctx.Context = authorizedbrand.WithResolved(ctx.Context, resolved)
	return nil
}
