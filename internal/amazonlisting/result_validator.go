package amazonlisting

import (
	"fmt"
	"sort"
	"strings"
)

type validator struct{}

func NewValidator() Validator {
	return &validator{}
}

func (v *validator) Validate(req *GenerateRequest, draft *AmazonListingDraft) *ValidationReport {
	report := &ValidationReport{Ready: true}
	if draft == nil {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "draft is nil")
		return report
	}

	v.validateTitle(report, draft)
	v.validateDescription(report, draft)
	v.validateBullets(report, draft)
	v.validateBrand(report, draft)
	v.validateCategory(report, draft)
	v.validateImages(report, req, draft)
	v.validatePricing(report, draft)
	v.validateVariants(report, draft)
	v.validateDimensions(report, draft)
	v.validateIPRisk(report, req, draft)
	v.mergeReviewSignals(report, draft)

	report.BlockingIssues = uniqueSorted(report.BlockingIssues)
	report.Warnings = uniqueSorted(report.Warnings)
	report.ReviewReasons = uniqueSorted(report.ReviewReasons)
	report.Ready = report.Ready && len(report.BlockingIssues) == 0
	return report
}

func (v *validator) validateTitle(report *ValidationReport, draft *AmazonListingDraft) {
	title := strings.TrimSpace(draft.Title)
	switch {
	case title == "":
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "title is required")
		appendReviewItem(draft, AmazonReviewItem{Field: "title", Action: OperatorActionEditTitle, Severity: "error", Reason: "title is required", IsBlocking: true, NeedsHuman: true, RecommendedFix: "fill a compliant marketplace title"})
	case len([]rune(title)) > 200:
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "title exceeds 200 characters")
		appendReviewItem(draft, AmazonReviewItem{Field: "title", Action: OperatorActionEditTitle, Severity: "error", Reason: "title exceeds 200 characters", IsBlocking: true, NeedsHuman: true, CurrentValue: title, RecommendedFix: "shorten the title to 200 characters or fewer"})
	case len([]rune(title)) < 20:
		report.Warnings = append(report.Warnings, "title is shorter than 20 characters")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "title may be too short for Amazon listing quality")
		appendReviewItem(draft, AmazonReviewItem{Field: "title", Action: OperatorActionEditTitle, Severity: "warning", Reason: "title may be too short for Amazon listing quality", NeedsHuman: true, CurrentValue: title, RecommendedFix: "expand the title with concrete product facts"})
	}
}

func (v *validator) validateDescription(report *ValidationReport, draft *AmazonListingDraft) {
	description := strings.TrimSpace(draft.Description)
	if description == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "description is required")
		appendReviewItem(draft, AmazonReviewItem{Field: "description", Action: OperatorActionManualReview, Severity: "error", Reason: "description is required", IsBlocking: true, NeedsHuman: true, RecommendedFix: "write a complete marketplace description"})
		return
	}
	if len([]rune(description)) < 80 {
		report.Warnings = append(report.Warnings, "description is shorter than 80 characters")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "description may be too short")
		appendReviewItem(draft, AmazonReviewItem{Field: "description", Action: OperatorActionManualReview, Severity: "warning", Reason: "description may be too short", NeedsHuman: true, CurrentValue: description, RecommendedFix: "expand the description with material, usage, size, and package details"})
	}
}

func (v *validator) validateBullets(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.BulletPoints) == 0 {
		report.Warnings = append(report.Warnings, "bullet_points are recommended")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing bullet points")
		appendReviewItem(draft, AmazonReviewItem{Field: "bullet_points", Action: OperatorActionFillBullets, Severity: "warning", Reason: "missing bullet points", NeedsHuman: true, RecommendedFix: "add 3-5 bullet points with key benefits"})
		return
	}
	if len(draft.BulletPoints) < 3 {
		report.Warnings = append(report.Warnings, "fewer than 3 bullet points")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "too few bullet points")
		appendReviewItem(draft, AmazonReviewItem{Field: "bullet_points", Action: OperatorActionEditBullets, Severity: "warning", Reason: "too few bullet points", NeedsHuman: true, RecommendedFix: "expand bullet points to cover key selling points"})
	}
	seen := make(map[string]struct{}, len(draft.BulletPoints))
	for _, bullet := range draft.BulletPoints {
		normalized := strings.ToLower(strings.TrimSpace(bullet))
		if normalized == "" {
			report.Warnings = append(report.Warnings, "empty bullet point detected")
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "empty bullet point detected")
			appendReviewItem(draft, AmazonReviewItem{Field: "bullet_points", Action: OperatorActionEditBullets, Severity: "warning", Reason: "empty bullet point detected", NeedsHuman: true, RecommendedFix: "remove or rewrite empty bullet points"})
			continue
		}
		if len([]rune(normalized)) > 250 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("bullet point exceeds 250 characters: %s", bullet))
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "bullet point too long")
			appendReviewItem(draft, AmazonReviewItem{Field: "bullet_points", Action: OperatorActionEditBullets, Severity: "warning", Reason: "bullet point too long", NeedsHuman: true, CurrentValue: bullet, RecommendedFix: "trim bullet points to 250 characters or fewer"})
		}
		if _, exists := seen[normalized]; exists {
			report.Warnings = append(report.Warnings, "duplicate bullet points detected")
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "duplicate bullet points detected")
			appendReviewItem(draft, AmazonReviewItem{Field: "bullet_points", Action: OperatorActionEditBullets, Severity: "warning", Reason: "duplicate bullet points detected", NeedsHuman: true, RecommendedFix: "deduplicate bullet points"})
			break
		}
		seen[normalized] = struct{}{}
	}
}

func (v *validator) validateBrand(report *ValidationReport, draft *AmazonListingDraft) {
	brand := strings.TrimSpace(draft.Brand)
	if brand == "" && draft.Attributes != nil {
		brand = strings.TrimSpace(draft.Attributes["brand"])
	}
	if brand == "" {
		report.Warnings = append(report.Warnings, "brand is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing brand")
		appendReviewItem(draft, AmazonReviewItem{Field: "brand", Action: OperatorActionFillBrand, Severity: "warning", Reason: "missing brand", NeedsHuman: true, RecommendedFix: "confirm or fill the selling brand"})
		return
	}
	badBrands := map[string]struct{}{
		"generic": {},
		"unknown": {},
		"n/a":     {},
	}
	if _, bad := badBrands[strings.ToLower(brand)]; bad {
		report.Warnings = append(report.Warnings, "brand is generic or placeholder")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "brand is generic or placeholder")
		appendReviewItem(draft, AmazonReviewItem{Field: "brand", Action: OperatorActionEditBrand, Severity: "warning", Reason: "brand is generic or placeholder", NeedsHuman: true, CurrentValue: brand, RecommendedFix: "replace placeholder brand with the actual brand"})
	}
}

func (v *validator) validateCategory(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.CategoryPath) == 0 {
		report.Warnings = append(report.Warnings, "category_path is empty")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing category mapping")
		appendReviewItem(draft, AmazonReviewItem{Field: "category_path", Action: OperatorActionEditCategory, Severity: "warning", Reason: "missing category mapping", NeedsHuman: true, RecommendedFix: "select the correct marketplace category"})
	}
}

func (v *validator) validateImages(report *ValidationReport, req *GenerateRequest, draft *AmazonListingDraft) {
	if req != nil && req.Options != nil && !req.Options.ProcessImages {
		return
	}
	if draft.Images == nil || strings.TrimSpace(draft.Images.MainImage) == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "main image is required")
		appendReviewItem(draft, AmazonReviewItem{Field: "images.main_image", Action: OperatorActionFillMainImage, Severity: "error", Reason: "main image is required", IsBlocking: true, NeedsHuman: true, RecommendedFix: "provide a valid compliant main image"})
	}
	if draft.Images == nil || strings.TrimSpace(draft.Images.WhiteBgImage) == "" {
		report.Warnings = append(report.Warnings, "white background image is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing white background image")
		appendReviewItem(draft, AmazonReviewItem{Field: "images.white_bg", Action: OperatorActionFillImages, Severity: "warning", Reason: "missing white background image", NeedsHuman: true, RecommendedFix: "generate or upload a compliant white background image"})
	}
	if draft.Images != nil && len(draft.Images.GalleryImages) == 0 {
		report.Warnings = append(report.Warnings, "gallery images are missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing gallery images")
		appendReviewItem(draft, AmazonReviewItem{Field: "images.gallery", Action: OperatorActionFillImages, Severity: "warning", Reason: "missing gallery images", NeedsHuman: true, RecommendedFix: "add supporting gallery images"})
	}
}

func (v *validator) validatePricing(report *ValidationReport, draft *AmazonListingDraft) {
	if draft.Pricing == nil {
		report.Warnings = append(report.Warnings, "pricing is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing pricing")
		appendReviewItem(draft, AmazonReviewItem{Field: "pricing", Action: OperatorActionFillPrice, Severity: "warning", Reason: "missing pricing", NeedsHuman: true, RecommendedFix: "fill suggested price, minimum price, and cost basis"})
		return
	}
	if strings.TrimSpace(draft.Pricing.Currency) == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "pricing currency is required")
		appendReviewItem(draft, AmazonReviewItem{Field: "pricing.currency", Action: OperatorActionEditPrice, Severity: "error", Reason: "pricing currency is required", IsBlocking: true, NeedsHuman: true, RecommendedFix: "set a valid marketplace currency"})
	}
	if draft.Pricing.SuggestedPrice < 0 || draft.Pricing.MinPrice < 0 || draft.Pricing.SourceCost < 0 {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "pricing contains negative values")
		appendReviewItem(draft, AmazonReviewItem{Field: "pricing", Action: OperatorActionEditPrice, Severity: "error", Reason: "pricing contains negative values", IsBlocking: true, NeedsHuman: true, RecommendedFix: "correct negative price values"})
	}
}

func (v *validator) validateVariants(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.Variants) == 0 {
		report.Warnings = append(report.Warnings, "variants are missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing variants")
		appendReviewItem(draft, AmazonReviewItem{Field: "variants", Action: OperatorActionFillSKU, Severity: "warning", Reason: "missing variants", NeedsHuman: true, RecommendedFix: "create at least one SKU/variant"})
		return
	}
	seenSKU := make(map[string]struct{}, len(draft.Variants))
	defaultCount := 0
	for _, variant := range draft.Variants {
		sku := strings.TrimSpace(variant.SKU)
		if sku == "" {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "variant SKU is required")
			appendReviewItem(draft, AmazonReviewItem{Field: "variants.sku", Action: OperatorActionFillSKU, Severity: "error", Reason: "variant SKU is required", IsBlocking: true, NeedsHuman: true, RecommendedFix: "fill a unique SKU for each variant"})
			continue
		}
		if _, exists := seenSKU[sku]; exists {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "duplicate variant SKU detected")
			appendReviewItem(draft, AmazonReviewItem{Field: "variants.sku", Action: OperatorActionEditSKU, Severity: "error", Reason: "duplicate variant SKU detected", IsBlocking: true, NeedsHuman: true, CurrentValue: sku, RecommendedFix: "make each variant SKU unique"})
		}
		seenSKU[sku] = struct{}{}
		if variant.IsDefault {
			defaultCount++
		}
		if variant.Price != nil && variant.Price.Amount < 0 {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "variant price cannot be negative")
			appendReviewItem(draft, AmazonReviewItem{Field: "variants.price", Action: OperatorActionEditPrice, Severity: "error", Reason: "variant price cannot be negative", IsBlocking: true, NeedsHuman: true, RecommendedFix: "correct negative variant prices"})
		}
	}
	if defaultCount == 0 {
		report.Warnings = append(report.Warnings, "default variant is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "default variant is missing")
		appendReviewItem(draft, AmazonReviewItem{Field: "variants.default", Action: OperatorActionEditSKU, Severity: "warning", Reason: "default variant is missing", NeedsHuman: true, RecommendedFix: "mark one variant as default"})
	}
	if defaultCount > 1 {
		report.Warnings = append(report.Warnings, "multiple default variants detected")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "multiple default variants detected")
		appendReviewItem(draft, AmazonReviewItem{Field: "variants.default", Action: OperatorActionEditSKU, Severity: "warning", Reason: "multiple default variants detected", NeedsHuman: true, RecommendedFix: "keep exactly one default variant"})
	}
}

func (v *validator) validateDimensions(report *ValidationReport, draft *AmazonListingDraft) {
	if draft.Dimensions != nil && strings.TrimSpace(draft.Dimensions.Unit) == "" {
		report.Warnings = append(report.Warnings, "dimensions unit is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "dimensions unit is missing")
		appendReviewItem(draft, AmazonReviewItem{Field: "dimensions.unit", Action: OperatorActionFillAttributes, Severity: "warning", Reason: "dimensions unit is missing", NeedsHuman: true, RecommendedFix: "fill the package/product dimension unit"})
	}
	if draft.Weight != nil && strings.TrimSpace(draft.Weight.Unit) == "" {
		report.Warnings = append(report.Warnings, "weight unit is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "weight unit is missing")
		appendReviewItem(draft, AmazonReviewItem{Field: "weight.unit", Action: OperatorActionFillAttributes, Severity: "warning", Reason: "weight unit is missing", NeedsHuman: true, RecommendedFix: "fill the product weight unit"})
	}
}

func (v *validator) validateIPRisk(report *ValidationReport, req *GenerateRequest, draft *AmazonListingDraft) {
	ipRisk := assessContentIPRisk(req, draft)
	draft.IPRisk = ipRisk
	draft.ListingIPRisk = mergeListingIPRisk(draft.ListingIPRisk, ipRisk)
	if ipRisk == nil {
		ipRisk = draft.ListingIPRisk
	}
	if ipRisk == nil {
		return
	}
	switch draft.ListingIPRisk.Level {
	case "high":
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "listing has high intellectual property risk")
		report.ReviewReasons = append(report.ReviewReasons, draft.ListingIPRisk.Reasons...)
		appendReviewItem(draft, AmazonReviewItem{Field: "ip_risk", Action: OperatorActionCheckCompliance, Severity: "error", Reason: "listing has high intellectual property risk", IsBlocking: true, NeedsHuman: true, RecommendedFix: "manually review trademark/copyright/patent risk"})
	case "medium":
		report.Warnings = append(report.Warnings, "listing may contain intellectual property risk signals")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, draft.ListingIPRisk.Reasons...)
		appendReviewItem(draft, AmazonReviewItem{Field: "ip_risk", Action: OperatorActionCheckCompliance, Severity: "warning", Reason: "listing may contain intellectual property risk signals", NeedsHuman: true, RecommendedFix: "manually review brand and IP risk before publish"})
	}
}

func (v *validator) mergeReviewSignals(report *ValidationReport, draft *AmazonListingDraft) {
	if draft.Review != nil && draft.Review.NeedsReview {
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, draft.Review.Reasons...)
	}
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
