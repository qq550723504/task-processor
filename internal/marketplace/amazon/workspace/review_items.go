package workspace

import (
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
	amazonmodel "task-processor/internal/marketplace/amazon/model"
)

func BuildReviewItemsFromCanonical(product *canonical.Product) []amazonmodel.AmazonReviewItem {
	if product == nil {
		return nil
	}

	var items []amazonmodel.AmazonReviewItem
	for field, trace := range product.FieldTraces {
		if !trace.NeedsReview {
			continue
		}
		items = append(items, amazonmodel.AmazonReviewItem{
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
		items = append(items, amazonmodel.AmazonReviewItem{
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
			items = append(items, amazonmodel.AmazonReviewItem{
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
		items = append(items, amazonmodel.AmazonReviewItem{
			Field:          "product",
			Action:         amazonmodel.OperatorActionManualReview,
			Severity:       "warning",
			Reason:         "canonical product still requires manual confirmation",
			Source:         "canonical_product",
			NeedsHuman:     true,
			RecommendedFix: "review key listing fields before publishing",
		})
	}
	return DedupeReviewItems(items)
}

func RefreshCanonicalReviewItems(items []amazonmodel.AmazonReviewItem, product *canonical.Product) []amazonmodel.AmazonReviewItem {
	if product == nil {
		return items
	}
	filtered := make([]amazonmodel.AmazonReviewItem, 0, len(items))
	for _, item := range items {
		switch item.Field {
		case "title", "brand", "category_path", "description", "selling_points", "seo_keywords", "attributes", "specifications", "product", "dimensions.unit", "weight.unit":
			continue
		default:
			if strings.HasPrefix(item.Field, "attributes.") {
				continue
			}
			if strings.HasPrefix(item.Field, "specifications.technical.") || strings.HasPrefix(item.Field, "dimensions.") || strings.HasPrefix(item.Field, "weight.") {
				continue
			}
			if strings.HasPrefix(item.Field, "package.") || strings.HasPrefix(item.Field, "variants[") {
				continue
			}
			filtered = append(filtered, item)
		}
	}
	return DedupeReviewItems(append(filtered, BuildReviewItemsFromCanonical(product)...))
}

func DedupeReviewItems(items []amazonmodel.AmazonReviewItem) []amazonmodel.AmazonReviewItem {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]amazonmodel.AmazonReviewItem, 0, len(items))
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
		return amazonmodel.OperatorActionFillBrand
	case "title":
		return amazonmodel.OperatorActionEditTitle
	case "category_path":
		return amazonmodel.OperatorActionEditCategory
	case "selling_points":
		return amazonmodel.OperatorActionFillBullets
	case "seo_keywords", "attributes", "specifications":
		return amazonmodel.OperatorActionFillAttributes
	default:
		if strings.HasPrefix(strings.TrimSpace(field), "specifications.technical.") ||
			strings.HasPrefix(strings.TrimSpace(field), "dimensions.") ||
			strings.HasPrefix(strings.TrimSpace(field), "weight.") ||
			strings.HasPrefix(strings.TrimSpace(field), "package.") {
			return amazonmodel.OperatorActionFillAttributes
		}
		if strings.HasPrefix(strings.TrimSpace(field), "attributes.") {
			return amazonmodel.OperatorActionFillAttributes
		}
		if strings.HasPrefix(strings.TrimSpace(field), "variants[") {
			if strings.Contains(field, ".sku") {
				return amazonmodel.OperatorActionEditSKU
			}
			if strings.Contains(field, ".price.") {
				return amazonmodel.OperatorActionEditPrice
			}
			return amazonmodel.OperatorActionFillAttributes
		}
		return amazonmodel.OperatorActionManualReview
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

func reviewEvidenceFromTrace(trace canonical.FieldTrace) []amazonmodel.AmazonReviewEvidence {
	if len(trace.Sources) == 0 {
		return nil
	}
	evidence := make([]amazonmodel.AmazonReviewEvidence, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		evidence = append(evidence, amazonmodel.AmazonReviewEvidence{
			Type:   string(source.Type),
			Detail: strings.TrimSpace(source.Detail),
		})
	}
	return evidence
}

func buildFieldEvidence(product *canonical.Product, field string, trace canonical.FieldTrace) []amazonmodel.AmazonReviewEvidence {
	evidence := reviewEvidenceFromTrace(trace)
	if snippet := fieldValueEvidence(product, field); snippet != "" {
		evidence = append(evidence, amazonmodel.AmazonReviewEvidence{
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

func parseIndexedField(field string, collection string) (int, string, bool) {
	prefix := collection + "["
	if !strings.HasPrefix(field, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(field, prefix)
	end := strings.Index(rest, "]")
	if end <= 0 || len(rest) <= end+1 || rest[end+1] != '.' {
		return 0, "", false
	}
	index := 0
	for _, ch := range rest[:end] {
		if ch < '0' || ch > '9' {
			return 0, "", false
		}
		index = index*10 + int(ch-'0')
	}
	return index, rest[end+2:], true
}
