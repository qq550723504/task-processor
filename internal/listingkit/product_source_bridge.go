package listingkit

import (
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/product/catalog"
)

// SourceFactsGenerateRequestInput is the narrow bridge input from normalized
// source facts into ListingKit orchestration. It must receive normalized catalog
// facts, not raw crawler payloads or source-specific DTOs.
type SourceFactsGenerateRequestInput struct {
	TenantID           string
	UserID             string
	Product            catalog.ProductFacts
	Platforms          []string
	Country            string
	Language           string
	SheinStoreID       int64
	TargetCategoryHint string
	Options            *GenerateOptions
}

// GenerateRequestFromSourceFacts converts normalized catalog/asset facts into
// the existing ListingKit GenerateRequest shape. It does not create tasks,
// submit packages, assemble marketplace payloads, or introduce new source-specific
// branches.
func GenerateRequestFromSourceFacts(input SourceFactsGenerateRequestInput) GenerateRequest {
	product := input.Product
	return GenerateRequest{
		TenantID:           strings.TrimSpace(input.TenantID),
		UserID:             strings.TrimSpace(input.UserID),
		ProductKey:         strings.TrimSpace(product.SourceKey),
		Text:               sourceFactsPromptText(product),
		Source:             sourceReferenceFromProductFacts(product),
		Platforms:          normalizedSourceFactsPlatforms(input.Platforms),
		Country:            strings.TrimSpace(input.Country),
		Language:           strings.TrimSpace(input.Language),
		SheinStoreID:       input.SheinStoreID,
		TargetCategoryHint: strings.TrimSpace(input.TargetCategoryHint),
		BrandHint:          strings.TrimSpace(product.Brand),
		Options:            input.Options,
	}
}

func sourceReferenceFromProductFacts(product catalog.ProductFacts) *SourceReference {
	if strings.TrimSpace(product.SourceKey) == "" &&
		strings.TrimSpace(product.SourceType) == "" &&
		strings.TrimSpace(product.SourcePlatform) == "" &&
		strings.TrimSpace(product.SourceID) == "" &&
		strings.TrimSpace(product.SourceURL) == "" {
		return nil
	}
	return &SourceReference{
		Key:      strings.TrimSpace(product.SourceKey),
		Type:     strings.TrimSpace(product.SourceType),
		Platform: strings.TrimSpace(product.SourcePlatform),
		ID:       strings.TrimSpace(product.SourceID),
		URL:      strings.TrimSpace(product.SourceURL),
	}
}

func normalizedSourceFactsPlatforms(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		platform = strings.ToLower(strings.TrimSpace(platform))
		if platform == "" {
			continue
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		out = append(out, platform)
	}
	return out
}

func sourceFactsPromptText(product catalog.ProductFacts) string {
	parts := []string{}
	appendPart := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, label+": "+value)
		}
	}
	appendPart("Title", product.Title)
	appendPart("Brand", product.Brand)
	appendPart("Description", product.Description)
	if len(product.Attributes) > 0 {
		keys := make([]string, 0, len(product.Attributes))
		for key := range product.Attributes {
			if strings.TrimSpace(key) != "" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendPart("Attribute "+key, product.Attributes[key])
		}
	}
	if len(product.Variants) > 0 {
		appendPart("Variant count", strconv.Itoa(len(product.Variants)))
	}
	return strings.Join(parts, "\n")
}
