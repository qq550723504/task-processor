package sourcing

import (
	"testing"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
)

func TestConvert1688ProductToScrapedDataMapsVariantDimensionsAndVariants(t *testing.T) {
	product := &alibaba1688model.Product1688{
		Title:    "Sneaker",
		Images:   []string{" https://example.com/main.jpg ", "", "https://example.com/main.jpg"},
		MinPrice: 29.9,
		Currency: "CNY",
		VariationsValues: []alibaba1688model.VariationValue{
			{VariantName: "颜色", Values: []string{"红色", "蓝色", "红色"}},
			{VariantName: "尺码", Values: []string{"42", "43"}},
		},
		Variants: []alibaba1688model.Variant{
			{
				Attributes: map[string]any{"颜色": "红色", "尺码": "42"},
				Image:      "https://example.com/red-42.jpg",
				Price:      35.5,
				Stock:      12,
			},
			{
				Attributes: map[string]any{"颜色": "蓝色", "尺码": "43"},
				Price:      36.5,
				Stock:      8,
			},
		},
	}

	scraped := Convert1688ProductToScrapedData(product)
	if scraped == nil {
		t.Fatal("Convert1688ProductToScrapedData() returned nil")
	}
	if len(scraped.VariantDimensions) != 2 {
		t.Fatalf("len(VariantDimensions) = %d, want 2", len(scraped.VariantDimensions))
	}
	if got := scraped.VariantDimensions[0].Name; got != "颜色" {
		t.Fatalf("VariantDimensions[0].Name = %q, want 颜色", got)
	}
	if len(scraped.VariantDimensions[0].Values) != 2 {
		t.Fatalf("len(VariantDimensions[0].Values) = %d, want 2", len(scraped.VariantDimensions[0].Values))
	}
	if len(scraped.Variants) != 2 {
		t.Fatalf("len(Variants) = %d, want 2", len(scraped.Variants))
	}
	if len(scraped.Images) != 1 || scraped.Images[0] != "https://example.com/main.jpg" {
		t.Fatalf("Images = %+v, want trimmed unique main image", scraped.Images)
	}
	if got := scraped.Variants[0].Attributes["颜色"]; got != "红色" {
		t.Fatalf("Variants[0].Attributes[颜色] = %q, want 红色", got)
	}
	if got := scraped.Variants[1].Images[0]; got != "https://example.com/main.jpg" {
		t.Fatalf("Variants[1].Images[0] = %q, want main image fallback", got)
	}
	if scraped.Variants[0].Price == nil || scraped.Variants[0].Price.Amount != 35.5 {
		t.Fatal("expected variant price to be mapped")
	}
}

func TestConvert1688ProductToScrapedDataCleansSpecsAndDescription(t *testing.T) {
	product := &alibaba1688model.Product1688{
		Title: "Fallback title",
		Specifications: []alibaba1688model.Specification{
			{Name: " Material ", Value: " Cotton "},
			{Name: " Empty ", Value: " "},
			{Name: " ", Value: "ignored"},
		},
		ProductDetails: []alibaba1688model.ProductDetail{
			{Content: "  "},
			{Content: " First line "},
			{Content: "\nSecond line\n"},
		},
	}

	scraped := Convert1688ProductToScrapedData(product)
	if scraped == nil {
		t.Fatal("Convert1688ProductToScrapedData() returned nil")
	}
	if got := scraped.Specs["Material"]; got != "Cotton" {
		t.Fatalf("Specs[Material] = %q, want Cotton", got)
	}
	if _, ok := scraped.Specs["Empty"]; ok {
		t.Fatalf("Specs contains empty value: %+v", scraped.Specs)
	}
	if got := scraped.Description; got != "First line\nSecond line" {
		t.Fatalf("Description = %q, want trimmed joined detail lines", got)
	}
}

func TestConvert1688ProductToScrapedDataDescriptionFallsBackToTitleForBlankDetails(t *testing.T) {
	product := &alibaba1688model.Product1688{
		Title:          "Fallback title",
		ProductDetails: []alibaba1688model.ProductDetail{{Content: "  "}},
	}

	scraped := Convert1688ProductToScrapedData(product)
	if scraped == nil {
		t.Fatal("Convert1688ProductToScrapedData() returned nil")
	}
	if scraped.Description != "Fallback title" {
		t.Fatalf("Description = %q, want title fallback", scraped.Description)
	}
}
