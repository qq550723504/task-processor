package amazonlisting

import (
	"fmt"
	"strings"

	amazonapi "task-processor/internal/amazon/api"
	amazonpublishing "task-processor/internal/marketplace/amazon/publishing"
)

type exportBuilder struct{}

func NewExportBuilder() ExportBuilder {
	return &exportBuilder{}
}

func (b *exportBuilder) Build(req *GenerateRequest, draft *AmazonListingDraft) *AmazonListingExport {
	if req == nil || draft == nil {
		return nil
	}

	sku := b.resolveSKU(draft)
	productType := b.resolveProductType(draft)
	marketplaceID := amazonpublishing.MarketplaceIDByCountry(req.Country)
	attributes := b.buildAttributes(req, draft, marketplaceID, sku)

	createReq := &amazonapi.ListingRequest{
		SKU:           sku,
		ProductType:   productType,
		Requirements:  "LISTING",
		Attributes:    attributes,
		MarketplaceID: marketplaceID,
	}
	previewReq := &amazonapi.ListingRequest{
		SKU:           sku,
		ProductType:   productType,
		Requirements:  "LISTING",
		Attributes:    amazonpublishing.CloneAttributes(attributes),
		MarketplaceID: marketplaceID,
	}
	updateReq := &amazonapi.ListingRequest{
		SKU:           sku,
		ProductType:   productType,
		Requirements:  "LISTING",
		Attributes:    amazonpublishing.CloneAttributes(attributes),
		MarketplaceID: marketplaceID,
	}
	patch := b.buildPatchPayload(sku, attributes)

	return &AmazonListingExport{
		ListingsAPI: &AmazonListingsAPIExport{
			SKU:                      sku,
			MarketplaceID:            marketplaceID,
			ProductType:              productType,
			Requirements:             "LISTING",
			Attributes:               attributes,
			ValidationPreviewRequest: previewReq,
			CreateRequest:            createReq,
			UpdateRequest:            updateReq,
			Patch:                    patch,
		},
	}
}

func (b *exportBuilder) resolveSKU(draft *AmazonListingDraft) string {
	for _, variant := range draft.Variants {
		if strings.TrimSpace(variant.SKU) != "" && variant.IsDefault {
			return strings.TrimSpace(variant.SKU)
		}
	}
	for _, variant := range draft.Variants {
		if strings.TrimSpace(variant.SKU) != "" {
			return strings.TrimSpace(variant.SKU)
		}
	}
	return amazonpublishing.SanitizeSKU(fmt.Sprintf("AL-%s", draft.TaskID))
}

func (b *exportBuilder) resolveProductType(draft *AmazonListingDraft) string {
	productType := strings.ToUpper(strings.TrimSpace(draft.ProductType))
	if productType != "" {
		return amazonpublishing.SanitizeProductType(productType)
	}
	if len(draft.CategoryPath) > 0 {
		return amazonpublishing.SanitizeProductType(strings.ToUpper(draft.CategoryPath[len(draft.CategoryPath)-1]))
	}
	return "PRODUCT"
}

func (b *exportBuilder) buildAttributes(req *GenerateRequest, draft *AmazonListingDraft, marketplaceID, sku string) map[string]any {
	attributes := make(map[string]any)

	if title := strings.TrimSpace(draft.Title); title != "" {
		attributes["item_name"] = []map[string]any{
			{"value": title, "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
	}
	if brand := strings.TrimSpace(draft.Brand); brand != "" {
		attributes["brand"] = []map[string]any{
			{"value": brand, "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
		attributes["manufacturer"] = []map[string]any{
			{"value": brand, "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
	}
	attributes["condition_type"] = []map[string]any{
		{"value": "new_new", "marketplace_id": marketplaceID},
	}
	if draft.Description != "" {
		attributes["product_description"] = []map[string]any{
			{"value": draft.Description, "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
	}
	if len(draft.BulletPoints) > 0 {
		points := make([]map[string]any, 0, len(draft.BulletPoints))
		for _, point := range draft.BulletPoints {
			point = strings.TrimSpace(point)
			if point == "" {
				continue
			}
			points = append(points, map[string]any{
				"value":          point,
				"language_tag":   amazonpublishing.NormalizeLanguageTag(req.Language),
				"marketplace_id": marketplaceID,
			})
		}
		if len(points) > 0 {
			attributes["bullet_point"] = points
		}
	}
	if len(draft.SearchTerms) > 0 {
		attributes["generic_keyword"] = []map[string]any{
			{"value": strings.Join(amazonpublishing.CompactStrings(draft.SearchTerms), " "), "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
	}
	if draft.Pricing != nil && draft.Pricing.SuggestedPrice > 0 {
		attributes["purchasable_offer"] = []map[string]any{
			{
				"audience":       "ALL",
				"currency":       normalizeCurrency(draft.Pricing.Currency, req.Country),
				"marketplace_id": marketplaceID,
				"our_price": []map[string]any{
					{
						"schedule": []map[string]any{
							{"value_with_tax": draft.Pricing.SuggestedPrice},
						},
					},
				},
			},
		}
	}

	quantity := 0
	if len(draft.Variants) > 0 {
		defaultVariant := draft.Variants[0]
		for _, variant := range draft.Variants {
			if variant.IsDefault {
				defaultVariant = variant
				break
			}
		}
		quantity = defaultVariant.Inventory
		if defaultVariant.Price != nil && draft.Pricing != nil && draft.Pricing.SuggestedPrice <= 0 {
			attributes["purchasable_offer"] = []map[string]any{
				{
					"audience":       "ALL",
					"currency":       normalizeCurrency(defaultVariant.Price.Currency, req.Country),
					"marketplace_id": marketplaceID,
					"our_price": []map[string]any{
						{
							"schedule": []map[string]any{
								{"value_with_tax": defaultVariant.Price.Amount},
							},
						},
					},
				},
			}
		}
		if strings.TrimSpace(sku) != "" {
			attributes["model_number"] = []map[string]any{
				{"value": sku, "marketplace_id": marketplaceID},
			}
		}
	}
	attributes["fulfillment_availability"] = []map[string]any{
		{"fulfillment_channel_code": "DEFAULT", "quantity": quantity},
	}

	if draft.Images != nil {
		if main := strings.TrimSpace(draft.Images.MainImage); main != "" {
			attributes["main_product_image_locator"] = []map[string]any{
				{"media_location": main, "marketplace_id": marketplaceID},
			}
		}
		if len(draft.Images.GalleryImages) > 0 {
			imageAttrs := make([]map[string]any, 0, len(draft.Images.GalleryImages))
			for _, image := range draft.Images.GalleryImages {
				image = strings.TrimSpace(image)
				if image == "" {
					continue
				}
				imageAttrs = append(imageAttrs, map[string]any{
					"media_location": image,
					"marketplace_id": marketplaceID,
				})
				if len(imageAttrs) == 8 {
					break
				}
			}
			if len(imageAttrs) > 0 {
				attributes["other_product_image_locator"] = imageAttrs
			}
		}
	}

	if draft.Dimensions != nil {
		attributes["item_dimensions"] = []map[string]any{
			{
				"marketplace_id": marketplaceID,
				"length": []map[string]any{
					{"value": draft.Dimensions.Length, "unit": amazonpublishing.NormalizeDimensionUnit(draft.Dimensions.Unit)},
				},
				"width": []map[string]any{
					{"value": draft.Dimensions.Width, "unit": amazonpublishing.NormalizeDimensionUnit(draft.Dimensions.Unit)},
				},
				"height": []map[string]any{
					{"value": draft.Dimensions.Height, "unit": amazonpublishing.NormalizeDimensionUnit(draft.Dimensions.Unit)},
				},
			},
		}
	}
	if draft.Weight != nil {
		attributes["item_weight"] = []map[string]any{
			{
				"value":          draft.Weight.Value,
				"unit":           amazonpublishing.NormalizeWeightUnit(draft.Weight.Unit),
				"marketplace_id": marketplaceID,
			},
		}
	}

	for key, value := range draft.Attributes {
		attrName := amazonpublishing.SanitizeAttributeName(key)
		if attrName == "" || value == "" {
			continue
		}
		if _, exists := attributes[attrName]; exists {
			continue
		}
		attributes[attrName] = []map[string]any{
			{"value": value, "language_tag": amazonpublishing.NormalizeLanguageTag(req.Language), "marketplace_id": marketplaceID},
		}
	}

	return attributes
}

func (b *exportBuilder) buildPatchPayload(sku string, attributes map[string]any) *AmazonListingsPatchPayload {
	if strings.TrimSpace(sku) == "" {
		return nil
	}
	patches := make([]AmazonListingsPatchAction, 0, len(attributes))
	for key, value := range attributes {
		if key == "" {
			continue
		}
		patches = append(patches, AmazonListingsPatchAction{
			Op:    "replace",
			Path:  "/attributes/" + key,
			Value: value,
		})
	}
	if len(patches) == 0 {
		return nil
	}
	return &AmazonListingsPatchPayload{
		SKU:     sku,
		Patches: patches,
	}
}
