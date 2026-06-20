package saleattribute

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheinpub "task-processor/internal/publishing/shein"
	productapi "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

type stubSaleAttributeRuntimeResolver struct {
	resolution *sheinpub.SaleAttributeResolution
	called     bool
}

func (s *stubSaleAttributeRuntimeResolver) Resolve(req *sheinpub.BuildRequest, canonicalProduct *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	s.called = true
	return s.resolution
}

func TestBuildCanonicalProductFromTaskContext(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	canonicalProduct, req, pkg, err := buildSaleAttributeResolutionInput(ctx)
	if err != nil {
		t.Fatalf("buildSaleAttributeResolutionInput() error = %v", err)
	}

	if canonicalProduct.Title != "Pet Bandana" {
		t.Fatalf("canonical title = %q, want Pet Bandana", canonicalProduct.Title)
	}
	if len(canonicalProduct.VariantDimensions) != 2 {
		t.Fatalf("variant dimensions = %d, want 2", len(canonicalProduct.VariantDimensions))
	}
	if canonicalProduct.VariantDimensions[0].Name != "Color" {
		t.Fatalf("first variant dimension = %q, want Color", canonicalProduct.VariantDimensions[0].Name)
	}
	if len(canonicalProduct.Variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(canonicalProduct.Variants))
	}
	if got := canonicalProduct.Variants[0].Attributes["Color"].Value; got != "Red" {
		t.Fatalf("first variant color = %q, want Red", got)
	}
	if req.SheinStoreID != 42 {
		t.Fatalf("shein store id = %d, want 42", req.SheinStoreID)
	}
	if req.Country != "US" {
		t.Fatalf("country = %q, want US", req.Country)
	}
	if pkg.CategoryID != 123 {
		t.Fatalf("package category id = %d, want 123", pkg.CategoryID)
	}
}

func TestSaleAttributeResolutionHandlerStoresSelectionOnTaskContext(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	valueID := 1001
	resolver := &stubSaleAttributeRuntimeResolver{
		resolution: &sheinpub.SaleAttributeResolution{
			Status:                   "resolved",
			Source:                   "runtime_resolver",
			PrimaryAttributeID:       27,
			SecondaryAttributeID:     87,
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
			SKUAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:            "sku",
				AttributeID:      87,
				AttributeValueID: &valueID,
				Value:            "M",
			}},
		},
	}

	handler := NewSaleAttributeResolutionHandler(resolver)
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if !resolver.called {
		t.Fatal("expected resolver to be called")
	}
	if ctx.SaleAttributeSelection == nil {
		t.Fatal("expected sale attribute selection to be stored on TaskContext")
	}
	if ctx.SaleAttributeSelection.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", ctx.SaleAttributeSelection.PrimaryAttributeID)
	}
	if ctx.SaleAttributeSelection.SecondaryAttributeID != 87 {
		t.Fatalf("secondary attribute id = %d, want 87", ctx.SaleAttributeSelection.SecondaryAttributeID)
	}
	if ctx.SaleAttributeSelection.Source != "runtime_resolver" {
		t.Fatalf("source = %q, want runtime_resolver", ctx.SaleAttributeSelection.Source)
	}
}

func TestSaleAttributeResolutionHandlerDropsUnresolvedSecondarySelection(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	resolver := &stubSaleAttributeRuntimeResolver{
		resolution: &sheinpub.SaleAttributeResolution{
			Status:                   "partial",
			Source:                   "runtime_resolver",
			PrimaryAttributeID:       27,
			SecondaryAttributeID:     87,
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
		},
	}

	handler := NewSaleAttributeResolutionHandler(resolver)
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if ctx.SaleAttributeSelection == nil {
		t.Fatal("expected primary sale attribute selection to be stored on TaskContext")
	}
	if ctx.SaleAttributeSelection.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", ctx.SaleAttributeSelection.PrimaryAttributeID)
	}
	if ctx.SaleAttributeSelection.SecondaryAttributeID != 0 {
		t.Fatalf("secondary attribute id = %d, want 0 when secondary mapping is unresolved", ctx.SaleAttributeSelection.SecondaryAttributeID)
	}
	if ctx.SaleAttributeSelection.SecondarySourceDimension != "" {
		t.Fatalf("secondary source dimension = %q, want empty when secondary mapping is unresolved", ctx.SaleAttributeSelection.SecondarySourceDimension)
	}
}

func TestSaleAttributeResolutionHandlerKeepsSecondarySelectionWhenSourceDimensionExists(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	resolver := &stubSaleAttributeRuntimeResolver{
		resolution: &sheinpub.SaleAttributeResolution{
			Status:                   "partial",
			Source:                   "runtime_resolver",
			PrimaryAttributeID:       27,
			SecondaryAttributeID:     87,
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
			SKUAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:       "sku",
				AttributeID: 87,
				Value:       "M",
			}},
		},
	}

	handler := NewSaleAttributeResolutionHandler(resolver)
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if ctx.SaleAttributeSelection == nil {
		t.Fatal("expected sale attribute selection to be stored on TaskContext")
	}
	if ctx.SaleAttributeSelection.SecondaryAttributeID != 87 {
		t.Fatalf("secondary attribute id = %d, want 87 when source dimension is available for downstream repair", ctx.SaleAttributeSelection.SecondaryAttributeID)
	}
	if ctx.SaleAttributeSelection.SecondarySourceDimension != "Size" {
		t.Fatalf("secondary source dimension = %q, want Size", ctx.SaleAttributeSelection.SecondarySourceDimension)
	}
}

func TestSaleAttributeResolutionHandlerFallsBackWhenResolverIsUnavailable(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	handler := NewSaleAttributeResolutionHandler(nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if ctx.SaleAttributeSelection != nil {
		t.Fatalf("selection = %+v, want nil fallback state", ctx.SaleAttributeSelection)
	}
}

func TestSaleAttributeResolutionHandlerSkipsSelectionWhenStoreContextMissing(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	ctx.SetStoreInfo(nil)
	resolver := &stubSaleAttributeRuntimeResolver{
		resolution: &sheinpub.SaleAttributeResolution{
			Status:                 "resolved",
			PrimaryAttributeID:     27,
			PrimarySourceDimension: "Color",
		},
	}

	handler := NewSaleAttributeResolutionHandler(resolver)
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v, want nil when runtime context is unavailable", err)
	}
	if resolver.called {
		t.Fatal("resolver should not be called when store context is missing")
	}
	if ctx.SaleAttributeSelection != nil {
		t.Fatalf("selection = %+v, want nil when runtime store context is missing", ctx.SaleAttributeSelection)
	}
}

func TestSaleAttributeResolutionHandlerReturnsErrorWhenCoreProductDataIsMissing(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	ctx.SetProductData(nil)
	resolver := &stubSaleAttributeRuntimeResolver{}
	handler := NewSaleAttributeResolutionHandler(resolver)

	if err := handler.Handle(ctx); err == nil {
		t.Fatal("expected error when product data is missing")
	}
	if resolver.called {
		t.Fatal("resolver should not be called when core product data is missing")
	}
}

func makeSaleResolutionTaskContext() *sheinctx.TaskContext {
	ctx := sheinctx.NewTaskContext(context.Background(), &model.Task{Region: "us"})
	ctx.SetAmazonProduct(&model.Product{
		Title:       "Pet Bandana",
		Brand:       "Miao",
		Description: "Soft adjustable pet scarf",
		Categories:  []string{"Pet Supplies", "Dog Apparel"},
		Images:      []string{"https://example.com/main.jpg", "https://example.com/detail.jpg"},
		VariationsValues: []model.VariationValue{
			{VariantName: "Color", Values: []string{"Red", "Blue"}},
			{VariantName: "Size", Values: []string{"S", "M"}},
		},
	})
	ctx.SetVariants([]model.Product{
		{
			Asin:     "A1",
			Title:    "red-s",
			ImageURL: "https://example.com/red.jpg",
			Variations: []model.Variation{{
				Asin:       "A1",
				Attributes: map[string]any{"Color": "Red", "Size": "S"},
			}},
		},
		{
			Asin:     "A2",
			Title:    "blue-m",
			ImageURL: "https://example.com/blue.jpg",
			Variations: []model.Variation{{
				Asin:       "A2",
				Attributes: map[string]any{"Color": "Blue", "Size": "M"},
			}},
		},
	})
	ctx.SetProductData(&productapi.Product{CategoryID: 123})
	ctx.SetStoreInfo(&listingruntime.StoreInfo{ID: 42, Region: "us"})
	return ctx
}
