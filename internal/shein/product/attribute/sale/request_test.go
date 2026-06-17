package sale

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/model"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func TestBuildUserPrompt_UsesBatchScopedVariationValuesAndVariants(t *testing.T) {
	builder := NewSaleAttributeRequestBuilder()
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Title: "Test Product",
			VariationsValues: []model.VariationValue{
				{VariantName: "Color", Values: []string{"Red", "Blue", "Green", "Black"}},
				{VariantName: "Size", Values: []string{"S", "M", "L", "XL"}},
			},
		},
		Variants: []model.Product{
			{Asin: "A1", ProductDetails: []model.ProductDetail{{Type: "Item Weight", Value: "100g"}}},
			{Asin: "A2", ProductDetails: []model.ProductDetail{{Type: "Item Weight", Value: "120g"}}},
			{Asin: "A3", ProductDetails: []model.ProductDetail{{Type: "Item Weight", Value: "140g"}}},
			{Asin: "A4", ProductDetails: []model.ProductDetail{{Type: "Item Weight", Value: "160g"}}},
		},
	}

	request := &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{
			{ASIN: "A1", Attributes: map[string]string{"color": "Red", "size": "S"}},
			{ASIN: "A2", Attributes: map[string]string{"color": "Blue", "size": "M"}},
		},
		VariationAttributeValues: &[]model.VariationValue{
			{VariantName: "Color", Values: []string{"Red", "Blue"}},
			{VariantName: "Size", Values: []string{"S", "M"}},
		},
		RequiredVariantCount: 2,
	}

	prompt := builder.BuildUserPrompt(input, request)

	if strings.Contains(prompt, `"Green"`) || strings.Contains(prompt, `"Black"`) {
		t.Fatalf("prompt contains out-of-batch color values: %s", prompt)
	}
	if strings.Contains(prompt, `"L"`) || strings.Contains(prompt, `"XL"`) {
		t.Fatalf("prompt contains out-of-batch size values: %s", prompt)
	}
	if strings.Contains(prompt, "A3") || strings.Contains(prompt, "A4") {
		t.Fatalf("prompt contains out-of-batch ASIN context: %s", prompt)
	}
	if !strings.Contains(prompt, "Generate 2 variants.") {
		t.Fatalf("prompt missing batch variant count: %s", prompt)
	}
}

func TestScopeVariationAttributeValuesToProductsData_FiltersByBatchUsage(t *testing.T) {
	values := &[]model.VariationValue{
		{VariantName: "Color", Values: []string{"Red", "Blue", "Green"}},
		{VariantName: "Size", Values: []string{"S", "M", "L"}},
	}
	productsData := []sheinattr.ProductVariantData{
		{ASIN: "A1", Attributes: map[string]string{"color": "Red", "size": "S"}},
		{ASIN: "A2", Attributes: map[string]string{"color": "Blue", "size": "M"}},
	}

	scoped := scopeVariationAttributeValuesToProductsData(values, productsData)

	if len(scoped) != 2 {
		t.Fatalf("scoped variation value count = %d, want 2", len(scoped))
	}
	if got := strings.Join(scoped[0].Values, ","); got != "Red,Blue" {
		t.Fatalf("scoped color values = %q, want %q", got, "Red,Blue")
	}
	if got := strings.Join(scoped[1].Values, ","); got != "S,M" {
		t.Fatalf("scoped size values = %q, want %q", got, "S,M")
	}
}

func TestBuildGenerationRequest_FiltersVariationDataToProductsDataASINs(t *testing.T) {
	builder := NewSaleAttributeRequestBuilder()
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Variations: []model.Variation{
				{Asin: "A1", Attributes: map[string]any{"Color": "Red"}},
				{Asin: "A2", Attributes: map[string]any{"Color": "Blue"}},
				{Asin: "A3", Attributes: map[string]any{"Color": "Green"}},
			},
			VariationsValues: []model.VariationValue{
				{VariantName: "Color", Values: []string{"Red", "Blue", "Green"}},
			},
		},
		Variants: []model.Product{
			{Asin: "A1"},
			{Asin: "A2"},
		},
	}

	request := builder.BuildGenerationRequest(
		input,
		[]map[string]string{
			{"asin": "A1", "title": "Variant A1", "color": "Red"},
			{"asin": "A2", "title": "Variant A2", "color": "Blue"},
		},
		nil,
		nil,
	)

	if got := len(request.ProductsData); got != 2 {
		t.Fatalf("products data count = %d, want 2", got)
	}
	if got := len(request.VariationData); got != 2 {
		t.Fatalf("variation data count = %d, want 2", got)
	}
	if request.VariationData[0].Asin != "A1" || request.VariationData[1].Asin != "A2" {
		t.Fatalf("variation data ASINs = [%s %s], want [A1 A2]", request.VariationData[0].Asin, request.VariationData[1].Asin)
	}
	if request.RequiredVariantCount != 2 {
		t.Fatalf("required variant count = %d, want 2", request.RequiredVariantCount)
	}
}

func TestBuildUserPrompt_UsesProductsDataCountWhenRequiredVariantCountMismatches(t *testing.T) {
	builder := NewSaleAttributeRequestBuilder()
	input := &SaleAttributeInput{
		Context: context.Background(),
		AmazonProduct: &model.Product{
			Title: "Test Product",
			VariationsValues: []model.VariationValue{
				{VariantName: "Color", Values: []string{"Red", "Blue"}},
			},
		},
		Variants: []model.Product{
			{Asin: "A1"},
			{Asin: "A2"},
			{Asin: "A3"},
			{Asin: "A4"},
			{Asin: "A5"},
			{Asin: "A6"},
		},
	}

	request := &sheinattr.GenerationRequest{
		ProductsData: []sheinattr.ProductVariantData{
			{ASIN: "A1", Attributes: map[string]string{"color": "Red"}},
			{ASIN: "A2", Attributes: map[string]string{"color": "Blue"}},
			{ASIN: "A3", Attributes: map[string]string{"color": "Red"}},
			{ASIN: "A4", Attributes: map[string]string{"color": "Blue"}},
			{ASIN: "A5", Attributes: map[string]string{"color": "Red"}},
			{ASIN: "A6", Attributes: map[string]string{"color": "Blue"}},
		},
		VariationAttributeValues: &[]model.VariationValue{
			{VariantName: "Color", Values: []string{"Red", "Blue"}},
		},
		RequiredVariantCount: 14,
	}

	prompt := builder.BuildUserPrompt(input, request)

	if !strings.Contains(prompt, "Task: generate SHEIN sale attributes for 6 Amazon products.") {
		t.Fatalf("prompt should use products data count in task header, got: %s", prompt)
	}
	if !strings.Contains(prompt, "This is a multi-variant product. Generate 6 variants.") {
		t.Fatalf("prompt should use products data count in variant hint, got: %s", prompt)
	}
}
