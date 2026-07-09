package listingkit

import (
	"sort"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

// SourceFactsGenerateRequestInput is the narrow bridge input from normalized
// source facts into ListingKit orchestration. It must receive catalog and asset
// facts, not raw crawler payloads or source-specific DTOs.
type SourceFactsGenerateRequestInput struct {
	TenantID           string
	UserID             string
	Product            catalog.ProductFacts
	Assets             asset.Facts
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
		ImageURLs:          imageURLsFromAssetFacts(input.Assets),
		Text:               sourceFactsPromptText(product),
		ProductURL:         strings.TrimSpace(product.SourceURL),
		Platforms:          normalizedSourceFactsPlatforms(input.Platforms),
		Country:            strings.TrimSpace(input.Country),
		Language:           strings.TrimSpace(input.Language),
		SheinStoreID:       input.SheinStoreID,
		TargetCategoryHint: sourceFactsCategoryHint(input.TargetCategoryHint, product),
		BrandHint:          strings.TrimSpace(product.Brand),
		Options:            input.Options,
	}
}

func imageURLsFromAssetFacts(facts asset.Facts) []string {
	if len(facts.Items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	urls := make([]string, 0, len(facts.Items))
	for _, item := range facts.Items {
		url := strings.TrimSpace(item.URL)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}
	return urls
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

func sourceFactsCategoryHint(explicit string, product catalog.ProductFacts) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if product.Attributes == nil {
		return ""
	}
	for _, key := range []string{"target_category", "category", "categories", "root_category"} {
		if value := strings.TrimSpace(product.Attributes[key]); value != "" {
			return value
		}
	}
	return ""
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
		appendPart("Variant count", strings.TrimSpace(stringFromInt(len(product.Variants))))
	}
	return strings.Join(parts, "\n")
}

func stringFromInt(value int) string {
	if value <= 0 {
		return ""
	}
	const digits = "0123456789"
	buf := make([]byte, 0, 10)
	for value > 0 {
		buf = append(buf, digits[value%10])
		value /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
