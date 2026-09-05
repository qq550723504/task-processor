package httpapi

import sourcea1688 "task-processor/internal/integration/crawler/a1688"

// productSnapshotRequest is the wire representation of a crawler snapshot.
// The sourcing model intentionally has no JSON tags, so the HTTP boundary
// must explicitly translate its snake_case protocol into the domain model.
type productSnapshotRequest struct {
	ID               string                          `json:"id"`
	Title            string                          `json:"title"`
	URL              string                          `json:"url"`
	Images           []string                        `json:"images"`
	MainImage        string                          `json:"main_image"`
	Videos           []videoSnapshotRequest          `json:"videos"`
	MinPrice         float64                         `json:"min_price"`
	MaxPrice         float64                         `json:"max_price"`
	Currency         string                          `json:"currency"`
	MinOrderQuantity int                             `json:"min_order_quantity"`
	Unit             string                          `json:"unit"`
	Supplier         supplierSnapshotRequest         `json:"supplier"`
	Specifications   []specificationSnapshotRequest  `json:"specifications"`
	ProductDetails   []productDetailSnapshotRequest  `json:"product_details"`
	PackInfo         *packInfoSnapshotRequest        `json:"pack_info"`
	VariationValues  []variationValueSnapshotRequest `json:"variation_values"`
	Variants         []variantSnapshotRequest        `json:"variants"`
	SalesVolume      int                             `json:"sales_volume"`
	ReviewCount      int                             `json:"review_count"`
	Rating           float64                         `json:"rating"`
	Shipping         shippingSnapshotRequest         `json:"shipping"`
	Category         string                          `json:"category"`
	Brand            string                          `json:"brand"`
	Keywords         []string                        `json:"keywords"`
	IsCustomized     bool                            `json:"is_customized"`
}

type videoSnapshotRequest struct {
	VideoURL string `json:"video_url"`
	CoverURL string `json:"cover_url"`
}
type supplierSnapshotRequest struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	CompanyName     string  `json:"company_name"`
	Location        string  `json:"location"`
	ShopURL         string  `json:"shop_url"`
	CardType        string  `json:"card_type"`
	YearsInBusiness int     `json:"years_in_business"`
	Rating          float64 `json:"rating"`
	ResponseRate    float64 `json:"response_rate"`
	IsGoldSupplier  bool    `json:"is_gold_supplier"`
	IsVerified      bool    `json:"is_verified"`
}
type specificationSnapshotRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type productDetailSnapshotRequest struct {
	Content string   `json:"content"`
	Images  []string `json:"images"`
}
type packInfoSnapshotRequest struct {
	PackageType   string   `json:"package_type"`
	Weight        float64  `json:"weight"`
	PackageImages []string `json:"package_images"`
	Instructions  string   `json:"instructions"`
}
type variationValueSnapshotRequest struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
type variantSnapshotRequest struct {
	Attributes map[string]any `json:"attributes"`
	Name       string         `json:"name"`
	Image      string         `json:"image"`
	Stock      int            `json:"stock"`
	Price      float64        `json:"price"`
}
type shippingSnapshotRequest struct {
	ShippingFrom   string `json:"shipping_from"`
	ProcessingTime string `json:"processing_time"`
}

func (r productSnapshotRequest) toSnapshot() sourcea1688.Alibaba1688ProductSnapshot {
	snapshot := sourcea1688.Alibaba1688ProductSnapshot{
		ID: r.ID, Title: r.Title, URL: r.URL, Images: r.Images, MainImage: r.MainImage,
		MinPrice: r.MinPrice, MaxPrice: r.MaxPrice,
		Currency: r.Currency, MinOrderQuantity: r.MinOrderQuantity, Unit: r.Unit,
		SalesVolume: r.SalesVolume, ReviewCount: r.ReviewCount, Rating: r.Rating,
		Category: r.Category, Brand: r.Brand, Keywords: r.Keywords, IsCustomized: r.IsCustomized,
		Shipping: sourcea1688.Alibaba1688ShippingSnapshot{ShippingFrom: r.Shipping.ShippingFrom, ProcessingTime: r.Shipping.ProcessingTime},
		Supplier: sourcea1688.Alibaba1688SupplierSnapshot{ID: r.Supplier.ID, Name: r.Supplier.Name, CompanyName: r.Supplier.CompanyName, Location: r.Supplier.Location, ShopURL: r.Supplier.ShopURL, CardType: r.Supplier.CardType, YearsInBusiness: r.Supplier.YearsInBusiness, Rating: r.Supplier.Rating, ResponseRate: r.Supplier.ResponseRate, IsGoldSupplier: r.Supplier.IsGoldSupplier, IsVerified: r.Supplier.IsVerified},
	}
	for _, video := range r.Videos {
		snapshot.Videos = append(snapshot.Videos, sourcea1688.Alibaba1688VideoSnapshot{VideoURL: video.VideoURL, CoverURL: video.CoverURL})
	}
	for _, spec := range r.Specifications {
		snapshot.Specifications = append(snapshot.Specifications, sourcea1688.Alibaba1688SpecificationSnapshot{Name: spec.Name, Value: spec.Value})
	}
	for _, detail := range r.ProductDetails {
		snapshot.ProductDetails = append(snapshot.ProductDetails, sourcea1688.Alibaba1688ProductDetailSnapshot{Content: detail.Content, Images: detail.Images})
	}
	for _, variation := range r.VariationValues {
		snapshot.VariationValues = append(snapshot.VariationValues, sourcea1688.Alibaba1688VariationValueSnapshot{Name: variation.Name, Values: variation.Values})
	}
	for _, variant := range r.Variants {
		snapshot.Variants = append(snapshot.Variants, sourcea1688.Alibaba1688VariantSnapshot{Attributes: variant.Attributes, Name: variant.Name, Image: variant.Image, Stock: variant.Stock, Price: variant.Price})
	}
	if r.PackInfo != nil {
		snapshot.PackInfo = &sourcea1688.Alibaba1688PackInfoSnapshot{PackageType: r.PackInfo.PackageType, Weight: r.PackInfo.Weight, PackageImages: r.PackInfo.PackageImages, Instructions: r.PackInfo.Instructions}
	}
	return snapshot
}
