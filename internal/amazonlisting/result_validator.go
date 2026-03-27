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
	case len([]rune(title)) > 200:
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "title exceeds 200 characters")
	case len([]rune(title)) < 20:
		report.Warnings = append(report.Warnings, "title is shorter than 20 characters")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "title may be too short for Amazon listing quality")
	}
}

func (v *validator) validateDescription(report *ValidationReport, draft *AmazonListingDraft) {
	description := strings.TrimSpace(draft.Description)
	if description == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "description is required")
		return
	}
	if len([]rune(description)) < 80 {
		report.Warnings = append(report.Warnings, "description is shorter than 80 characters")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "description may be too short")
	}
}

func (v *validator) validateBullets(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.BulletPoints) == 0 {
		report.Warnings = append(report.Warnings, "bullet_points are recommended")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing bullet points")
		return
	}
	if len(draft.BulletPoints) < 3 {
		report.Warnings = append(report.Warnings, "fewer than 3 bullet points")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "too few bullet points")
	}
	seen := make(map[string]struct{}, len(draft.BulletPoints))
	for _, bullet := range draft.BulletPoints {
		normalized := strings.ToLower(strings.TrimSpace(bullet))
		if normalized == "" {
			report.Warnings = append(report.Warnings, "empty bullet point detected")
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "empty bullet point detected")
			continue
		}
		if len([]rune(normalized)) > 250 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("bullet point exceeds 250 characters: %s", bullet))
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "bullet point too long")
		}
		if _, exists := seen[normalized]; exists {
			report.Warnings = append(report.Warnings, "duplicate bullet points detected")
			report.NeedsReview = true
			report.ReviewReasons = append(report.ReviewReasons, "duplicate bullet points detected")
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
	}
}

func (v *validator) validateCategory(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.CategoryPath) == 0 {
		report.Warnings = append(report.Warnings, "category_path is empty")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing category mapping")
	}
}

func (v *validator) validateImages(report *ValidationReport, req *GenerateRequest, draft *AmazonListingDraft) {
	if req != nil && req.Options != nil && !req.Options.ProcessImages {
		return
	}
	if draft.Images == nil || strings.TrimSpace(draft.Images.MainImage) == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "main image is required")
	}
	if draft.Images == nil || strings.TrimSpace(draft.Images.WhiteBgImage) == "" {
		report.Warnings = append(report.Warnings, "white background image is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing white background image")
	}
	if draft.Images != nil && len(draft.Images.GalleryImages) == 0 {
		report.Warnings = append(report.Warnings, "gallery images are missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing gallery images")
	}
}

func (v *validator) validatePricing(report *ValidationReport, draft *AmazonListingDraft) {
	if draft.Pricing == nil {
		report.Warnings = append(report.Warnings, "pricing is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing pricing")
		return
	}
	if strings.TrimSpace(draft.Pricing.Currency) == "" {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "pricing currency is required")
	}
	if draft.Pricing.SuggestedPrice < 0 || draft.Pricing.MinPrice < 0 || draft.Pricing.SourceCost < 0 {
		report.Ready = false
		report.BlockingIssues = append(report.BlockingIssues, "pricing contains negative values")
	}
}

func (v *validator) validateVariants(report *ValidationReport, draft *AmazonListingDraft) {
	if len(draft.Variants) == 0 {
		report.Warnings = append(report.Warnings, "variants are missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "missing variants")
		return
	}
	seenSKU := make(map[string]struct{}, len(draft.Variants))
	defaultCount := 0
	for _, variant := range draft.Variants {
		sku := strings.TrimSpace(variant.SKU)
		if sku == "" {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "variant SKU is required")
			continue
		}
		if _, exists := seenSKU[sku]; exists {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "duplicate variant SKU detected")
		}
		seenSKU[sku] = struct{}{}
		if variant.IsDefault {
			defaultCount++
		}
		if variant.Price != nil && variant.Price.Amount < 0 {
			report.Ready = false
			report.BlockingIssues = append(report.BlockingIssues, "variant price cannot be negative")
		}
	}
	if defaultCount == 0 {
		report.Warnings = append(report.Warnings, "default variant is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "default variant is missing")
	}
	if defaultCount > 1 {
		report.Warnings = append(report.Warnings, "multiple default variants detected")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "multiple default variants detected")
	}
}

func (v *validator) validateDimensions(report *ValidationReport, draft *AmazonListingDraft) {
	if draft.Dimensions != nil && strings.TrimSpace(draft.Dimensions.Unit) == "" {
		report.Warnings = append(report.Warnings, "dimensions unit is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "dimensions unit is missing")
	}
	if draft.Weight != nil && strings.TrimSpace(draft.Weight.Unit) == "" {
		report.Warnings = append(report.Warnings, "weight unit is missing")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, "weight unit is missing")
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
	case "medium":
		report.Warnings = append(report.Warnings, "listing may contain intellectual property risk signals")
		report.NeedsReview = true
		report.ReviewReasons = append(report.ReviewReasons, draft.ListingIPRisk.Reasons...)
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
