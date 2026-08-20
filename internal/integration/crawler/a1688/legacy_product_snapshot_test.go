package a1688

import (
	"reflect"
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
)

func TestSnapshotFromLegacyProductNil(t *testing.T) {
	if got := SnapshotFromLegacyProduct(nil); got != nil {
		t.Fatalf("SnapshotFromLegacyProduct(nil) = %#v, want nil", got)
	}
}

func TestSnapshotFromLegacyProductMapsConsumedFields(t *testing.T) {
	legacy := populatedLegacyProduct()
	snapshot := SnapshotFromLegacyProduct(legacy)
	if snapshot == nil {
		t.Fatal("SnapshotFromLegacyProduct() returned nil")
	}

	if snapshot.ID != legacy.ID || snapshot.Title != legacy.Title || snapshot.URL != legacy.URL ||
		snapshot.MainImage != legacy.MainImage || snapshot.PriceRangeCount != 2 ||
		snapshot.MinPrice != legacy.MinPrice || snapshot.MaxPrice != legacy.MaxPrice ||
		snapshot.Currency != legacy.Currency || snapshot.MinOrderQuantity != legacy.MinOrderQuantity ||
		snapshot.Unit != legacy.Unit || snapshot.SalesVolume != legacy.SalesVolume ||
		snapshot.ReviewCount != legacy.ReviewCount || snapshot.Rating != legacy.Rating ||
		snapshot.Category != legacy.Category || snapshot.Brand != legacy.Brand ||
		snapshot.IsCustomized != legacy.IsCustomized {
		t.Fatalf("snapshot scalar fields = %#v, want values copied from legacy product", snapshot)
	}
	if !reflect.DeepEqual(snapshot.Images, legacy.Images) || !reflect.DeepEqual(snapshot.Keywords, legacy.Keywords) {
		t.Fatalf("snapshot top-level slices = %#v/%#v, want %#v/%#v", snapshot.Images, snapshot.Keywords, legacy.Images, legacy.Keywords)
	}
	if len(snapshot.Videos) != 1 || snapshot.Videos[0].VideoURL != legacy.Videos[0].VideoURL || snapshot.Videos[0].CoverURL != legacy.Videos[0].CoverURL {
		t.Fatalf("snapshot videos = %#v, want video URLs copied", snapshot.Videos)
	}
	if snapshot.Supplier.ID != legacy.Supplier.ID || snapshot.Supplier.Name != legacy.Supplier.Name ||
		snapshot.Supplier.CompanyName != legacy.Supplier.CompanyName || snapshot.Supplier.Location != legacy.Supplier.Location ||
		snapshot.Supplier.ShopURL != legacy.Supplier.ShopURL || snapshot.Supplier.CardType != legacy.Supplier.CardType ||
		snapshot.Supplier.YearsInBusiness != legacy.Supplier.YearsInBusiness || snapshot.Supplier.Rating != legacy.Supplier.Rating ||
		snapshot.Supplier.ResponseRate != legacy.Supplier.ResponseRate || snapshot.Supplier.IsGoldSupplier != legacy.Supplier.IsGoldSupplier ||
		snapshot.Supplier.IsVerified != legacy.Supplier.IsVerified {
		t.Fatalf("snapshot supplier = %#v, want supplier facts copied", snapshot.Supplier)
	}
	if len(snapshot.Specifications) != 1 || snapshot.Specifications[0].Name != "Material" || snapshot.Specifications[0].Value != "Oxford cloth" {
		t.Fatalf("snapshot specifications = %#v, want copied specification", snapshot.Specifications)
	}
	if len(snapshot.ProductDetails) != 1 || snapshot.ProductDetails[0].Content != legacy.ProductDetails[0].Content ||
		!reflect.DeepEqual(snapshot.ProductDetails[0].Images, legacy.ProductDetails[0].Images) {
		t.Fatalf("snapshot product details = %#v, want copied details", snapshot.ProductDetails)
	}
	if snapshot.PackInfo == nil || snapshot.PackInfo.PackageType != legacy.PackInfo.PackageType ||
		snapshot.PackInfo.Weight != legacy.PackInfo.Weight || snapshot.PackInfo.Instructions != legacy.PackInfo.Instructions ||
		!reflect.DeepEqual(snapshot.PackInfo.PackageImages, legacy.PackInfo.PackageImages) {
		t.Fatalf("snapshot pack info = %#v, want copied pack facts", snapshot.PackInfo)
	}
	if len(snapshot.VariationValues) != 1 || snapshot.VariationValues[0].Name != "Color" ||
		!reflect.DeepEqual(snapshot.VariationValues[0].Values, []string{"Red", "Blue"}) {
		t.Fatalf("snapshot variation values = %#v, want copied variation values", snapshot.VariationValues)
	}
	if len(snapshot.Variants) != 1 || snapshot.Variants[0].Name != legacy.Variants[0].Name ||
		snapshot.Variants[0].Image != legacy.Variants[0].Image || snapshot.Variants[0].Stock != legacy.Variants[0].Stock ||
		snapshot.Variants[0].Price != legacy.Variants[0].Price || !reflect.DeepEqual(snapshot.Variants[0].Attributes, legacy.Variants[0].Attributes) {
		t.Fatalf("snapshot variants = %#v, want copied variants", snapshot.Variants)
	}
	if snapshot.Shipping.ShippingFrom != legacy.ShippingInfo.ShippingFrom || snapshot.Shipping.ProcessingTime != legacy.ShippingInfo.ProcessingTime {
		t.Fatalf("snapshot shipping = %#v, want copied shipping facts", snapshot.Shipping)
	}
}

func TestSnapshotFromLegacyProductDeepCopiesMutableFields(t *testing.T) {
	legacy := populatedLegacyProduct()
	snapshot := SnapshotFromLegacyProduct(legacy)

	legacy.Images[0] = "mutated-image"
	legacy.Keywords[0] = "mutated-keyword"
	legacy.ProductDetails[0].Images[0] = "mutated-detail"
	legacy.PackInfo.PackageImages[0] = "mutated-package"
	legacy.VariationsValues[0].Values[0] = "mutated-value"
	legacy.Variants[0].Attributes["Color"] = "mutated-color"
	legacy.Variants[0] = model.Variant{Name: "mutated-variant"}

	if snapshot.Images[0] != "image-1" || snapshot.Keywords[0] != "keyword-1" ||
		snapshot.ProductDetails[0].Images[0] != "detail-image-1" || snapshot.PackInfo.PackageImages[0] != "package-image-1" ||
		snapshot.VariationValues[0].Values[0] != "Red" || snapshot.Variants[0].Attributes["Color"] != "Red" ||
		snapshot.Variants[0].Name != "Black" {
		t.Fatalf("snapshot changed after mutating legacy product: %#v", snapshot)
	}
}

func populatedLegacyProduct() *model.Product1688 {
	return &model.Product1688{
		ID:               "1688-321",
		Title:            "Insulated Lunch Bag",
		URL:              "https://detail.1688.com/offer/321.html",
		Images:           []string{"image-1", "image-2"},
		MainImage:        "main-image",
		Videos:           []model.Video{{VideoURL: "video-1", CoverURL: "cover-1"}},
		PriceRanges:      []model.PriceRange{{MinQuantity: 1, Price: 18.8}, {MinQuantity: 10, Price: 17.2}},
		MinPrice:         17.2,
		MaxPrice:         18.8,
		Currency:         "CNY",
		MinOrderQuantity: 3,
		Unit:             "piece",
		Supplier: model.SupplierInfo{
			ID: "supplier-321", Name: "Lunch Factory", CompanyName: "Lunch Factory Co.", Location: "Zhejiang",
			ShopURL: "https://shop.example/321", CardType: "factory", YearsInBusiness: 8,
			Rating: 4.8, ResponseRate: 0.99, IsGoldSupplier: true, IsVerified: true,
		},
		Specifications: []model.Specification{{Name: "Material", Value: "Oxford cloth"}},
		ProductDetails: []model.ProductDetail{{Content: "Thermal lunch bag", Images: []string{"detail-image-1"}}},
		PackInfo: &model.PackInfo{
			PackageType: "carton", Weight: 320, PackageImages: []string{"package-image-1"}, Instructions: "Keep dry",
		},
		VariationsValues: []model.VariationValue{{VariantName: "Color", Values: []string{"Red", "Blue"}}},
		Variants: []model.Variant{{
			Attributes: map[string]any{"Color": "Red"}, Name: "Black", Image: "variant-image", Stock: 50, Price: 19.9,
		}},
		SalesVolume: 1200,
		ReviewCount: 86,
		Rating:      4.7,
		ShippingInfo: model.ShippingInfo{
			ShippingFrom: "Yiwu", ProcessingTime: "3 days",
		},
		Category:     "Bags",
		Brand:        "Factory Lunch",
		Keywords:     []string{"keyword-1", "keyword-2"},
		IsCustomized: true,
	}
}
