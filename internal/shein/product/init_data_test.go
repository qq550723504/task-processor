package product

import (
	"context"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/shein/authorizedbrand"
	sheinctx "task-processor/internal/shein/context"
)

type stubAuthorizedBrandResolver struct {
	resolved     *authorizedbrand.Resolved
	err          error
	gotConfig    authorizedbrand.Config
	gotBrandName string
	callCount    int
}

func (s *stubAuthorizedBrandResolver) ResolveForProductBrand(_ context.Context, cfg authorizedbrand.Config, productBrand string) (*authorizedbrand.Resolved, error) {
	s.callCount++
	s.gotConfig = cfg
	s.gotBrandName = productBrand
	return s.resolved, s.err
}

func TestInitProductDataHandlerSetsAuthorizedBrandCode(t *testing.T) {
	handler := NewInitProductDataHandler()
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			AuthorizedBrand: &authorizedbrand.Resolved{
				Enabled: true,
				Code:    "2fd1n",
				Name:    "Logitech",
			},
		},
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if ctx.ProductData == nil {
		t.Fatal("ProductData = nil, want initialized product")
	}
	if ctx.ProductData.BrandCode == nil || *ctx.ProductData.BrandCode != "2fd1n" {
		t.Fatalf("ProductData.BrandCode = %#v, want 2fd1n", ctx.ProductData.BrandCode)
	}
}

func TestEnsureAuthorizedBrandResolvedWithResolver_UsesAmazonBrandAndStoresResolution(t *testing.T) {
	enabled := true
	ctx := sheinctx.NewTaskContext(context.Background(), &model.Task{StoreID: 1})
	ctx.StoreInfo = &managementapi.StoreRespDTO{
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "fallback-code",
		AuthorizedBrandName:      "Fallback",
	}
	ctx.AmazonProduct = &model.Product{Brand: "Nike"}

	resolver := &stubAuthorizedBrandResolver{
		resolved: &authorizedbrand.Resolved{
			Enabled: true,
			Code:    "nike-1",
			Name:    "Nike",
			NameEn:  "Nike",
		},
	}

	if err := ensureAuthorizedBrandResolvedWithResolver(ctx, authorizedbrand.ConfigFromStore(ctx.StoreInfo), resolver); err != nil {
		t.Fatalf("ensureAuthorizedBrandResolvedWithResolver() error = %v", err)
	}
	if resolver.callCount != 1 {
		t.Fatalf("ResolveForProductBrand() call count = %d, want 1", resolver.callCount)
	}
	if resolver.gotBrandName != "Nike" {
		t.Fatalf("ResolveForProductBrand() productBrand = %q, want Nike", resolver.gotBrandName)
	}
	if ctx.AuthorizedBrand == nil || ctx.AuthorizedBrand.Code != "nike-1" {
		t.Fatalf("AuthorizedBrand = %+v, want nike-1", ctx.AuthorizedBrand)
	}
	fromCtx, ok := authorizedbrand.FromContext(ctx.Context)
	if !ok || fromCtx.Code != "nike-1" {
		t.Fatalf("authorizedbrand.FromContext() = %+v, %v, want nike-1, true", fromCtx, ok)
	}
}

func TestInitProductDataHandler_ResolvesAuthorizedBrandBeforeSettingBrandCode(t *testing.T) {
	handler := NewInitProductDataHandler()
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			Context: context.Background(),
			StoreInfo: &managementapi.StoreRespDTO{
				EnableBrandAuthorization: boolPtr(true),
			},
		},
		ProductState: sheinctx.ProductState{
			AmazonProduct: &model.Product{Brand: "Nike"},
		},
	}
	resolver := &stubAuthorizedBrandResolver{
		resolved: &authorizedbrand.Resolved{
			Enabled: true,
			Code:    "nike-1",
			Name:    "Nike",
			NameEn:  "Nike",
		},
	}

	if err := ensureAuthorizedBrandResolvedWithResolver(ctx, authorizedbrand.ConfigFromStore(ctx.StoreInfo), resolver); err != nil {
		t.Fatalf("ensureAuthorizedBrandResolvedWithResolver() error = %v", err)
	}
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if ctx.ProductData == nil || ctx.ProductData.BrandCode == nil || *ctx.ProductData.BrandCode != "nike-1" {
		t.Fatalf("ProductData.BrandCode = %#v, want nike-1", ctx.ProductData)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
