package sale_test

import (
	"strings"
	"testing"

	"task-processor/internal/model"
	"task-processor/internal/shein/product/attribute/sale"
)

func newContextBuilder() *sale.SaleAttributeContextBuilder {
	return sale.NewSaleAttributeContextBuilder()
}

func TestSaleAttributeContextBuilder_BuildCompactProductContext(t *testing.T) {
	c := newContextBuilder()

	tests := []struct {
		name         string
		product      model.Product
		variants     []model.Product
		wantContains []string
	}{
		{
			"full_product",
			model.Product{
				Title:      "Test Product",
				Brand:      "TestBrand",
				Categories: []string{"Electronics", "Gadgets"},
				Features:   []string{"Feature1", "Feature2", "Feature3", "Feature4"},
			},
			[]model.Product{{}, {}},
			[]string{"Test Product", "TestBrand", "Electronics > Gadgets", "Feature1", "变体数: 2"},
		},
		{
			"only_title",
			model.Product{Title: "Simple Product"},
			nil,
			[]string{"Simple Product", "变体数: 1"},
		},
		{
			"description_when_no_features",
			model.Product{
				Title:       "Product",
				Description: "This is a long description that should be truncated",
			},
			nil,
			[]string{"Product", "描述:"},
		},
		{
			"features_truncated_to_3",
			model.Product{
				Title:    "Product",
				Features: []string{"F1", "F2", "F3", "F4", "F5"},
			},
			nil,
			[]string{"F1", "F2", "F3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.BuildCompactProductContext(tc.product, tc.variants)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildCompactProductContext() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestSaleAttributeContextBuilder_BuildCompactProductContext_DescriptionTruncated(t *testing.T) {
	c := newContextBuilder()

	longDesc := strings.Repeat("a", 300)
	product := model.Product{Title: "P", Description: longDesc}

	got := c.BuildCompactProductContext(product, nil)
	// 描述应该被截断到 200 字符 + "..."
	if strings.Contains(got, longDesc) {
		t.Error("description should be truncated but was not")
	}
	if !strings.Contains(got, "...") {
		t.Error("truncated description should end with '...'")
	}
}

func TestSaleAttributeContextBuilder_BuildCompactProductContext_EmptyProduct(t *testing.T) {
	c := newContextBuilder()

	got := c.BuildCompactProductContext(model.Product{}, nil)
	// 空产品只有变体数
	if !strings.Contains(got, "变体数: 1") {
		t.Errorf("expected '变体数: 1' in output, got: %q", got)
	}
}
