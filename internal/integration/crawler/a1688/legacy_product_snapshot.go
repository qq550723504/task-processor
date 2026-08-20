package a1688

import (
	"slices"

	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/product/sourcing"
)

// SnapshotFromLegacyProduct converts the legacy crawler DTO into the narrow
// product-sourcing contract. All mutable fields are copied so product-domain
// normalization cannot mutate crawler-owned data.
func SnapshotFromLegacyProduct(product *model.Product1688) *sourcing.Alibaba1688ProductSnapshot {
	if product == nil {
		return nil
	}

	snapshot := &sourcing.Alibaba1688ProductSnapshot{
		ID:               product.ID,
		Title:            product.Title,
		URL:              product.URL,
		Images:           slices.Clone(product.Images),
		MainImage:        product.MainImage,
		PriceRangeCount:  len(product.PriceRanges),
		MinPrice:         product.MinPrice,
		MaxPrice:         product.MaxPrice,
		Currency:         product.Currency,
		MinOrderQuantity: product.MinOrderQuantity,
		Unit:             product.Unit,
		Supplier: sourcing.Alibaba1688SupplierSnapshot{
			ID:              product.Supplier.ID,
			Name:            product.Supplier.Name,
			CompanyName:     product.Supplier.CompanyName,
			Location:        product.Supplier.Location,
			ShopURL:         product.Supplier.ShopURL,
			CardType:        product.Supplier.CardType,
			YearsInBusiness: product.Supplier.YearsInBusiness,
			Rating:          product.Supplier.Rating,
			ResponseRate:    product.Supplier.ResponseRate,
			IsGoldSupplier:  product.Supplier.IsGoldSupplier,
			IsVerified:      product.Supplier.IsVerified,
		},
		SalesVolume: product.SalesVolume,
		ReviewCount: product.ReviewCount,
		Rating:      product.Rating,
		Shipping: sourcing.Alibaba1688ShippingSnapshot{
			ShippingFrom:   product.ShippingInfo.ShippingFrom,
			ProcessingTime: product.ShippingInfo.ProcessingTime,
		},
		Category:     product.Category,
		Brand:        product.Brand,
		Keywords:     slices.Clone(product.Keywords),
		IsCustomized: product.IsCustomized,
	}

	snapshot.Videos = make([]sourcing.Alibaba1688VideoSnapshot, len(product.Videos))
	for index, video := range product.Videos {
		snapshot.Videos[index] = sourcing.Alibaba1688VideoSnapshot{
			VideoURL: video.VideoURL,
			CoverURL: video.CoverURL,
		}
	}

	snapshot.Specifications = make([]sourcing.Alibaba1688SpecificationSnapshot, len(product.Specifications))
	for index, specification := range product.Specifications {
		snapshot.Specifications[index] = sourcing.Alibaba1688SpecificationSnapshot{
			Name:  specification.Name,
			Value: specification.Value,
		}
	}

	snapshot.ProductDetails = make([]sourcing.Alibaba1688ProductDetailSnapshot, len(product.ProductDetails))
	for index, detail := range product.ProductDetails {
		snapshot.ProductDetails[index] = sourcing.Alibaba1688ProductDetailSnapshot{
			Content: detail.Content,
			Images:  slices.Clone(detail.Images),
		}
	}

	if product.PackInfo != nil {
		snapshot.PackInfo = &sourcing.Alibaba1688PackInfoSnapshot{
			PackageType:   product.PackInfo.PackageType,
			Weight:        product.PackInfo.Weight,
			PackageImages: slices.Clone(product.PackInfo.PackageImages),
			Instructions:  product.PackInfo.Instructions,
		}
	}

	snapshot.VariationValues = make([]sourcing.Alibaba1688VariationValueSnapshot, len(product.VariationsValues))
	for index, variation := range product.VariationsValues {
		snapshot.VariationValues[index] = sourcing.Alibaba1688VariationValueSnapshot{
			Name:   variation.VariantName,
			Values: slices.Clone(variation.Values),
		}
	}

	snapshot.Variants = make([]sourcing.Alibaba1688VariantSnapshot, len(product.Variants))
	for index, variant := range product.Variants {
		attributes := make(map[string]any, len(variant.Attributes))
		for key, value := range variant.Attributes {
			attributes[key] = value
		}
		snapshot.Variants[index] = sourcing.Alibaba1688VariantSnapshot{
			Attributes: attributes,
			Name:       variant.Name,
			Image:      variant.Image,
			Stock:      variant.Stock,
			Price:      variant.Price,
		}
	}

	return snapshot
}
