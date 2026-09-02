package workspace

import (
	"fmt"
	"strings"

	amazonmodel "task-processor/internal/marketplace/amazon/model"
	"task-processor/internal/product/catalog"
)

func BuildReviewItemsFromSnapshot(snapshot *catalog.ProductSnapshot) []amazonmodel.AmazonReviewItem {
	if snapshot == nil {
		return nil
	}

	items := make([]amazonmodel.AmazonReviewItem, 0)
	for _, attribute := range snapshot.Attributes {
		if !attribute.Trace.NeedsReview {
			continue
		}
		field := "attributes." + strings.TrimSpace(attribute.Name)
		items = append(items, reviewItemFromTrace(field, attribute.Value, attribute.Trace))
	}
	for index, variant := range snapshot.Variants {
		if variant.Trace.NeedsReview {
			field := fmt.Sprintf("variants[%d]", index)
			items = append(items, reviewItemFromTrace(field, variant.Title, variant.Trace))
		}
		for _, attribute := range variant.Attributes {
			if !attribute.Trace.NeedsReview {
				continue
			}
			field := fmt.Sprintf("variants[%d].attributes.%s", index, strings.TrimSpace(attribute.Name))
			items = append(items, reviewItemFromTrace(field, attribute.Value, attribute.Trace))
		}
	}
	if snapshot.Review != nil && snapshot.Review.NeedsReview {
		for _, reason := range snapshot.Review.Reasons {
			reason = strings.TrimSpace(reason)
			if reason == "" {
				continue
			}
			items = append(items, amazonmodel.AmazonReviewItem{
				Field:          "product",
				Action:         amazonmodel.OperatorActionManualReview,
				Severity:       "warning",
				Reason:         reason,
				Source:         "product_snapshot",
				NeedsHuman:     true,
				RecommendedFix: "review the product snapshot evidence before publishing",
			})
		}
	}
	return DedupeReviewItems(items)
}

func reviewItemFromTrace(field, currentValue string, trace catalog.Trace) amazonmodel.AmazonReviewItem {
	return amazonmodel.AmazonReviewItem{
		Field:          field,
		Action:         reviewActionForField(field),
		Severity:       "warning",
		Reason:         fmt.Sprintf("%s has low confidence (%.2f)", field, trace.Confidence),
		Source:         reviewSourceFromTrace(trace),
		NeedsHuman:     true,
		CurrentValue:   currentValue,
		RecommendedFix: reviewRecommendationForField(field),
		Confidence:     trace.Confidence,
		IsInferred:     trace.IsInferred,
		Evidence:       reviewEvidenceFromTrace(trace),
	}
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
	if strings.HasPrefix(strings.TrimSpace(field), "attributes.") || strings.Contains(field, ".attributes.") {
		return amazonmodel.OperatorActionFillAttributes
	}
	return amazonmodel.OperatorActionManualReview
}

func reviewRecommendationForField(field string) string {
	if strings.HasPrefix(strings.TrimSpace(field), "attributes.") || strings.Contains(field, ".attributes.") {
		return "confirm the product attribute against its source evidence"
	}
	return "review and confirm this product fact manually"
}

func reviewSourceFromTrace(trace catalog.Trace) string {
	if len(trace.Sources) == 0 {
		return "unknown"
	}
	parts := make([]string, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		parts = append(parts, strings.TrimSpace(source.Type))
	}
	return strings.Join(parts, ",")
}

func reviewEvidenceFromTrace(trace catalog.Trace) []amazonmodel.AmazonReviewEvidence {
	if len(trace.Sources) == 0 {
		return nil
	}
	evidence := make([]amazonmodel.AmazonReviewEvidence, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		evidence = append(evidence, amazonmodel.AmazonReviewEvidence{
			Type:   strings.TrimSpace(source.Type),
			Detail: strings.TrimSpace(source.Detail),
		})
	}
	return evidence
}
