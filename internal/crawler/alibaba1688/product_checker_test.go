package alibaba1688

import (
	"strings"
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
)

func newValidProductForValidation() *model.Product1688 {
	return &model.Product1688{
		URL:              "https://detail.1688.com/offer/1.html",
		Title:            "Insulated lunch bag",
		MinPrice:         18.8,
		MinOrderQuantity: 1,
		Supplier:         model.SupplierInfo{Name: "Lunch Factory"},
		MainImage:        "https://img.example/product.jpg",
	}
}

func TestValidatePricingAcceptsAscendingPriceRanges(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		MinPrice:         7.5,
		MaxPrice:         12.5,
		MinOrderQuantity: 1,
		PriceRanges: []model.PriceRange{
			{MinQuantity: 1, MaxQuantity: 9, Price: 12.5},
			{MinQuantity: 10, MaxQuantity: 49, Price: 9.9},
			{MinQuantity: 50, MaxQuantity: 0, Price: 7.5},
		},
	}

	if err := checker.validatePricing(product); err != nil {
		t.Fatalf("expected sorted price ranges to pass validation, got %v", err)
	}
}

func TestValidatePricingRejectsDuplicateMinQuantity(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{
		MinPrice:         7.5,
		MaxPrice:         12.5,
		MinOrderQuantity: 1,
		PriceRanges: []model.PriceRange{
			{MinQuantity: 1, MaxQuantity: 9, Price: 12.5},
			{MinQuantity: 1, MaxQuantity: 0, Price: 9.9},
		},
	}

	if err := checker.validatePricing(product); err == nil {
		t.Fatal("expected duplicate min quantity to fail validation")
	}
}

func TestValidateProductRejectsMissingRequiredFields(t *testing.T) {
	checker := NewProductChecker()
	product := &model.Product1688{URL: "https://detail.1688.com/offer/1.html", MainImage: "https://img.example/product.jpg"}

	if err := checker.ValidateProduct(product); err == nil {
		t.Fatal("ValidateProduct() error = nil, want missing required fields error")
	}
}

func TestValidateProductAcceptsCompleteRequiredFields(t *testing.T) {
	checker := NewProductChecker()
	product := newValidProductForValidation()

	if err := checker.ValidateProduct(product); err != nil {
		t.Fatalf("ValidateProduct() error = %v, want complete product accepted", err)
	}
}

func TestValidateProductRejectsHostlessOptionalImageURL(t *testing.T) {
	checker := NewProductChecker()
	product := newValidProductForValidation()
	product.Variants = []model.Variant{{Image: "https:///.jpg"}}

	if err := checker.ValidateProduct(product); err == nil {
		t.Fatal("ValidateProduct() error = nil, want hostless optional image URL rejected")
	}
}

func TestValidateProductRejectsMediaURLsWithUserInfoWithoutEchoingCredentials(t *testing.T) {
	checker := NewProductChecker()
	badURL := credentialedExternalURLForTest()

	tests := []struct {
		name       string
		mutate     func(*model.Product1688)
		wantDetail string
	}{
		{
			name: "main image",
			mutate: func(product *model.Product1688) {
				product.MainImage = badURL
			},
			wantDetail: "主图URL格式无效",
		},
		{
			name: "gallery image",
			mutate: func(product *model.Product1688) {
				product.Images = []string{badURL}
			},
			wantDetail: "图片[0]URL格式无效",
		},
		{
			name: "video url",
			mutate: func(product *model.Product1688) {
				product.Videos = []model.Video{{VideoURL: badURL}}
			},
			wantDetail: "视频[0]URL格式无效",
		},
		{
			name: "video cover",
			mutate: func(product *model.Product1688) {
				product.Videos = []model.Video{{CoverURL: badURL}}
			},
			wantDetail: "视频[0]封面URL格式无效",
		},
		{
			name: "detail image",
			mutate: func(product *model.Product1688) {
				product.ProductDetails = []model.ProductDetail{{Images: []string{badURL}}}
			},
			wantDetail: "详情[0]图片[0]URL格式无效",
		},
		{
			name: "variant image",
			mutate: func(product *model.Product1688) {
				product.Variants = []model.Variant{{Image: badURL}}
			},
			wantDetail: "变体[0]图片URL格式无效",
		},
		{
			name: "package image",
			mutate: func(product *model.Product1688) {
				product.PackInfo = &model.PackInfo{PackageImages: []string{badURL}}
			},
			wantDetail: "包装图片[0]URL格式无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := newValidProductForValidation()
			tt.mutate(product)

			err := checker.ValidateProduct(product)
			if err == nil {
				t.Fatal("ValidateProduct() error = nil, want invalid media URL rejected")
			}
			if !strings.Contains(err.Error(), tt.wantDetail) {
				t.Fatal("validation error did not identify the failing field")
			}
			if strings.Contains(err.Error(), credentialedUserInfoForTest()) {
				t.Fatal("validation error echoed credential-bearing URL data")
			}
		})
	}
}
