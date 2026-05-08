package amazonlisting

import (
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func buildReviewItemsFromCanonical(product *canonical.Product) []AmazonReviewItem {
	if product == nil {
		return nil
	}

	var items []AmazonReviewItem
	for field, trace := range product.FieldTraces {
		if !trace.NeedsReview {
			continue
		}
		items = append(items, AmazonReviewItem{
			Field:          field,
			Action:         reviewActionForField(field),
			Severity:       "warning",
			Reason:         fmt.Sprintf("%s has low confidence (%.2f)", field, trace.Confidence),
			Source:         reviewSourceFromTrace(trace),
			NeedsHuman:     true,
			CurrentValue:   canonicalFieldValue(product, field),
			RecommendedFix: reviewRecommendationForField(field),
			Confidence:     trace.Confidence,
			IsInferred:     trace.IsInferred,
			Evidence:       buildFieldEvidence(product, field, trace),
		})
	}
	for key, attr := range product.Attributes {
		if !attr.Trace.NeedsReview {
			continue
		}
		field := "attributes." + strings.TrimSpace(key)
		items = append(items, AmazonReviewItem{
			Field:          field,
			Action:         reviewActionForField(field),
			Severity:       "warning",
			Reason:         fmt.Sprintf("%s has low confidence (%.2f)", field, attr.Trace.Confidence),
			Source:         reviewSourceFromTrace(attr.Trace),
			NeedsHuman:     true,
			CurrentValue:   attr.Value,
			RecommendedFix: reviewRecommendationForField(field),
			Confidence:     attr.Trace.Confidence,
			IsInferred:     attr.Trace.IsInferred,
			Evidence:       buildFieldEvidence(product, field, attr.Trace),
		})
	}
	for idx, variant := range product.Variants {
		for key, attr := range variant.Attributes {
			if !attr.Trace.NeedsReview {
				continue
			}
			field := fmt.Sprintf("variants[%d].attributes.%s", idx, strings.TrimSpace(key))
			items = append(items, AmazonReviewItem{
				Field:          field,
				Action:         reviewActionForField(field),
				Severity:       "warning",
				Reason:         fmt.Sprintf("%s has low confidence (%.2f)", field, attr.Trace.Confidence),
				Source:         reviewSourceFromTrace(attr.Trace),
				NeedsHuman:     true,
				CurrentValue:   attr.Value,
				RecommendedFix: reviewRecommendationForField(field),
				Confidence:     attr.Trace.Confidence,
				IsInferred:     attr.Trace.IsInferred,
				Evidence:       buildFieldEvidence(product, field, attr.Trace),
			})
		}
	}

	if product.NeedsReview && len(items) == 0 {
		items = append(items, AmazonReviewItem{
			Field:          "product",
			Action:         OperatorActionManualReview,
			Severity:       "warning",
			Reason:         "canonical product still requires manual confirmation",
			Source:         "canonical_product",
			NeedsHuman:     true,
			RecommendedFix: "review key listing fields before publishing",
		})
	}
	return dedupeReviewItems(items)
}

func appendReviewItem(draft *AmazonListingDraft, item AmazonReviewItem) {
	if draft == nil || strings.TrimSpace(item.Reason) == "" {
		return
	}
	draft.ReviewItems = dedupeReviewItems(append(draft.ReviewItems, item))
}

func dedupeReviewItems(items []AmazonReviewItem) []AmazonReviewItem {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]AmazonReviewItem, 0, len(items))
	for _, item := range items {
		key := strings.Join([]string{
			strings.TrimSpace(item.Field),
			strings.TrimSpace(item.Action),
			strings.TrimSpace(item.Reason),
			strings.TrimSpace(item.Source),
		}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func reviewActionForField(field string) string {
	switch strings.TrimSpace(field) {
	case "brand":
		return OperatorActionFillBrand
	case "title":
		return OperatorActionEditTitle
	case "category_path":
		return OperatorActionEditCategory
	case "selling_points":
		return OperatorActionFillBullets
	case "seo_keywords", "attributes", "specifications":
		return OperatorActionFillAttributes
	default:
		if strings.HasPrefix(strings.TrimSpace(field), "specifications.technical.") ||
			strings.HasPrefix(strings.TrimSpace(field), "dimensions.") ||
			strings.HasPrefix(strings.TrimSpace(field), "weight.") ||
			strings.HasPrefix(strings.TrimSpace(field), "package.") {
			return OperatorActionFillAttributes
		}
		if strings.HasPrefix(strings.TrimSpace(field), "attributes.") {
			return OperatorActionFillAttributes
		}
		if strings.HasPrefix(strings.TrimSpace(field), "variants[") {
			if strings.Contains(field, ".sku") {
				return OperatorActionEditSKU
			}
			if strings.Contains(field, ".price.") {
				return OperatorActionEditPrice
			}
			return OperatorActionFillAttributes
		}
		return OperatorActionManualReview
	}
}

func reviewRecommendationForField(field string) string {
	switch strings.TrimSpace(field) {
	case "brand":
		return "confirm the supplier brand or replace with your selling brand"
	case "title":
		return "rewrite the title to match marketplace style and real product facts"
	case "category_path":
		return "map the product to the correct marketplace category"
	case "selling_points":
		return "补齐 3-5 条明确卖点"
	case "seo_keywords":
		return "补充真实搜索词，避免泛词"
	case "attributes", "specifications":
		return "补齐关键属性和规格参数"
	default:
		if strings.HasPrefix(strings.TrimSpace(field), "specifications.technical.") ||
			strings.HasPrefix(strings.TrimSpace(field), "dimensions.") ||
			strings.HasPrefix(strings.TrimSpace(field), "weight.") ||
			strings.HasPrefix(strings.TrimSpace(field), "package.") {
			return "补齐并确认规格参数"
		}
		if strings.HasPrefix(strings.TrimSpace(field), "attributes.") {
			return "确认并补齐该商品属性"
		}
		if strings.HasPrefix(strings.TrimSpace(field), "variants[") {
			if strings.Contains(field, ".sku") {
				return "为该变体补齐并确认唯一 SKU"
			}
			if strings.Contains(field, ".price.") {
				return "确认该变体价格和币种"
			}
			return "确认该变体属性和展示信息"
		}
		return "review and confirm this field manually"
	}
}

func reviewSourceFromTrace(trace canonical.FieldTrace) string {
	if len(trace.Sources) == 0 {
		return "unknown"
	}
	parts := make([]string, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		parts = append(parts, string(source.Type))
	}
	return strings.Join(parts, ",")
}

func reviewEvidenceFromTrace(trace canonical.FieldTrace) []AmazonReviewEvidence {
	if len(trace.Sources) == 0 {
		return nil
	}
	evidence := make([]AmazonReviewEvidence, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		evidence = append(evidence, AmazonReviewEvidence{
			Type:   string(source.Type),
			Detail: strings.TrimSpace(source.Detail),
		})
	}
	return evidence
}

func buildFieldEvidence(product *canonical.Product, field string, trace canonical.FieldTrace) []AmazonReviewEvidence {
	evidence := reviewEvidenceFromTrace(trace)
	if snippet := fieldValueEvidence(product, field); snippet != "" {
		evidence = append(evidence, AmazonReviewEvidence{
			Type:   "field_value",
			Detail: snippet,
		})
	}
	return evidence
}

func fieldValueEvidence(product *canonical.Product, field string) string {
	value := strings.TrimSpace(canonicalFieldValue(product, field))
	if value == "" {
		return ""
	}
	if len(value) > 120 {
		value = value[:117] + "..."
	}
	return field + ` = "` + value + `"`
}

func canonicalFieldValue(product *canonical.Product, field string) string {
	if product == nil {
		return ""
	}
	switch field {
	case "title":
		return product.Title
	case "brand":
		return product.Brand
	case "category_path":
		return strings.Join(product.CategoryPath, " > ")
	case "description":
		return product.Description
	case "selling_points":
		return strings.Join(product.SellingPoints, " | ")
	case "seo_keywords":
		return strings.Join(product.SEOKeywords, ", ")
	case "specifications":
		if product.Specifications == nil {
			return ""
		}
		return "specifications"
	default:
		if strings.HasPrefix(field, "specifications.technical.") {
			key := strings.TrimPrefix(field, "specifications.technical.")
			if product.Specifications != nil && product.Specifications.Technical != nil {
				return product.Specifications.Technical[key]
			}
		}
		if strings.HasPrefix(field, "dimensions.") && product.Specifications != nil && product.Specifications.Dimensions != nil {
			switch strings.TrimPrefix(field, "dimensions.") {
			case "length":
				return fmt.Sprintf("%.2f", product.Specifications.Dimensions.Length)
			case "width":
				return fmt.Sprintf("%.2f", product.Specifications.Dimensions.Width)
			case "height":
				return fmt.Sprintf("%.2f", product.Specifications.Dimensions.Height)
			case "unit":
				return product.Specifications.Dimensions.Unit
			}
		}
		if strings.HasPrefix(field, "weight.") && product.Specifications != nil && product.Specifications.Weight != nil {
			switch strings.TrimPrefix(field, "weight.") {
			case "value":
				return fmt.Sprintf("%.2f", product.Specifications.Weight.Value)
			case "unit":
				return product.Specifications.Weight.Unit
			}
		}
		if strings.HasPrefix(field, "package.") && product.Specifications != nil && product.Specifications.Package != nil {
			switch strings.TrimPrefix(field, "package.") {
			case "quantity":
				return fmt.Sprintf("%d", product.Specifications.Package.Quantity)
			case "dimensions.length":
				if product.Specifications.Package.Dimensions != nil {
					return fmt.Sprintf("%.2f", product.Specifications.Package.Dimensions.Length)
				}
			case "dimensions.width":
				if product.Specifications.Package.Dimensions != nil {
					return fmt.Sprintf("%.2f", product.Specifications.Package.Dimensions.Width)
				}
			case "dimensions.height":
				if product.Specifications.Package.Dimensions != nil {
					return fmt.Sprintf("%.2f", product.Specifications.Package.Dimensions.Height)
				}
			case "dimensions.unit":
				if product.Specifications.Package.Dimensions != nil {
					return product.Specifications.Package.Dimensions.Unit
				}
			case "weight.value":
				if product.Specifications.Package.Weight != nil {
					return fmt.Sprintf("%.2f", product.Specifications.Package.Weight.Value)
				}
			case "weight.unit":
				if product.Specifications.Package.Weight != nil {
					return product.Specifications.Package.Weight.Unit
				}
			}
		}
		if strings.HasPrefix(field, "attributes.") {
			key := strings.TrimPrefix(field, "attributes.")
			if attr, ok := product.Attributes[key]; ok {
				return attr.Value
			}
		}
		if index, subfield, ok := parseIndexedField(field, "variants"); ok && index < len(product.Variants) {
			variant := product.Variants[index]
			switch {
			case subfield == "sku":
				return variant.SKU
			case subfield == "barcode":
				return variant.Barcode
			case subfield == "inventory":
				return fmt.Sprintf("%d", variant.Stock)
			case subfield == "is_default":
				if variant.IsDefault {
					return "true"
				}
				return "false"
			case subfield == "main_image":
				if len(variant.Images) > 0 {
					return variant.Images[0].URL
				}
			case subfield == "price.amount":
				if variant.Price != nil {
					return fmt.Sprintf("%.2f", variant.Price.Amount)
				}
			case subfield == "price.currency":
				if variant.Price != nil {
					return variant.Price.Currency
				}
			case strings.HasPrefix(subfield, "attributes."):
				key := strings.TrimPrefix(subfield, "attributes.")
				if attr, ok := variant.Attributes[key]; ok {
					return attr.Value
				}
			}
		}
		return ""
	}
}
