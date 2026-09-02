package amazonlisting

import (
	"fmt"
	"strconv"
	"strings"
)

func removeResolvedReviewItems(items []AmazonReviewItem, edits []DraftFieldEdit) []AmazonReviewItem {
	if len(items) == 0 || len(edits) == 0 {
		return items
	}
	edited := make(map[string]struct{}, len(edits))
	for _, edit := range edits {
		for _, field := range relatedReviewFields(strings.TrimSpace(edit.Field)) {
			edited[field] = struct{}{}
		}
	}
	filtered := make([]AmazonReviewItem, 0, len(items))
	for _, item := range items {
		if _, ok := edited[item.Field]; ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func trimStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func ensureDraftImages(draft *AmazonListingDraft) {
	if draft.Images == nil {
		draft.Images = &AmazonImageBundle{}
	}
}

func ensureDraftPricing(draft *AmazonListingDraft) {
	if draft.Pricing == nil {
		draft.Pricing = &AmazonPricingDraft{}
	}
}

func ensureDraftDimensions(draft *AmazonListingDraft) {
	if draft.Dimensions == nil {
		draft.Dimensions = &AmazonDimensions{}
	}
}

func ensureDraftWeight(draft *AmazonListingDraft) {
	if draft.Weight == nil {
		draft.Weight = &AmazonWeight{}
	}
}

func ensureDraftPackage(draft *AmazonListingDraft) {
	if draft.Package == nil {
		draft.Package = &AmazonPackageInfo{}
	}
}

func ensureDraftPackageDimensions(draft *AmazonListingDraft) {
	ensureDraftPackage(draft)
	if draft.Package.Dimensions == nil {
		draft.Package.Dimensions = &AmazonDimensions{}
	}
}

func ensureDraftPackageWeight(draft *AmazonListingDraft) {
	ensureDraftPackage(draft)
	if draft.Package.Weight == nil {
		draft.Package.Weight = &AmazonWeight{}
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
	index, err := strconv.Atoi(rest[:end])
	if err != nil || index < 0 {
		return 0, "", false
	}
	return index, rest[end+2:], true
}

func parseBooleanEdit(edit DraftFieldEdit, field string) (bool, error) {
	value := strings.TrimSpace(strings.ToLower(edit.StringValue))
	switch value {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s requires string_value true/false", field)
	}
}

func relatedReviewFields(field string) []string {
	fields := []string{field}
	if _, subfield, ok := parseIndexedField(field, "variants"); ok {
		switch subfield {
		case "sku":
			fields = append(fields, "variants.sku")
		case "is_default":
			fields = append(fields, "variants.default")
		case "price.amount", "price.currency", "cost_price.amount", "cost_price.currency":
			fields = append(fields, "variants.price")
		}
	}
	return fields
}

func validateRequest(req *GenerateRequest) error {
	if req.Marketplace == "" {
		req.Marketplace = "amazon"
	}
	if req.Marketplace != "amazon" {
		return fmt.Errorf("only amazon marketplace is supported currently")
	}
	if strings.TrimSpace(req.ProductKey) == "" {
		return fmt.Errorf("product_key is required")
	}
	return nil
}
