package amazonlisting

import (
	"strings"
	"time"
)

type AutoFixer interface {
	Fix(req *GenerateRequest, draft *AmazonListingDraft)
	FixIssues(req *GenerateRequest, draft *AmazonListingDraft, issues []AmazonIssue) []AmazonFixRecord
}

type autoFixer struct{}

func NewAutoFixer() AutoFixer {
	return &autoFixer{}
}

func (f *autoFixer) Fix(req *GenerateRequest, draft *AmazonListingDraft) {
	if draft == nil {
		return
	}

	f.fixBrand(req, draft)
	f.fixTitle(draft)
	f.fixBulletPoints(draft)
	f.fixPricing(req, draft)
	f.fixVariants(draft)
	f.fixImages(draft)
}

func (f *autoFixer) FixIssues(req *GenerateRequest, draft *AmazonListingDraft, issues []AmazonIssue) []AmazonFixRecord {
	if draft == nil || len(issues) == 0 {
		return nil
	}

	history := make([]AmazonFixRecord, 0, len(issues))
	for _, issue := range issues {
		record := AmazonFixRecord{
			At:      time.Now(),
			Issue:   issue.Type,
			Success: true,
		}
		switch issue.Type {
		case "missing_brand", "invalid_brand":
			before := draft.Brand
			f.fixBrand(req, draft)
			record.Action = "fill_brand"
			record.Success = strings.TrimSpace(draft.Brand) != "" && draft.Brand != before || strings.TrimSpace(draft.Brand) != ""
		case "title_too_long", "invalid_title":
			before := draft.Title
			f.fixTitle(draft)
			record.Action = "trim_title"
			record.Success = strings.TrimSpace(draft.Title) != "" && len([]rune(draft.Title)) <= 200 && draft.Title != before || len([]rune(draft.Title)) <= 200
		case "missing_bullet", "invalid_bullet":
			before := len(draft.BulletPoints)
			f.fixBulletPoints(draft)
			record.Action = "rebuild_bullets"
			record.Success = len(draft.BulletPoints) >= before && len(draft.BulletPoints) > 0
		case "missing_main_image", "missing_image":
			before := ""
			if draft.Images != nil {
				before = draft.Images.MainImage
			}
			f.fixImages(draft)
			record.Action = "fill_main_image"
			record.Success = draft.Images != nil && strings.TrimSpace(draft.Images.MainImage) != "" && draft.Images.MainImage != before || draft.Images != nil && strings.TrimSpace(draft.Images.MainImage) != ""
		case "missing_price", "invalid_price":
			before := ""
			if draft.Pricing != nil {
				before = draft.Pricing.Currency
			}
			f.fixPricing(req, draft)
			record.Action = "fill_pricing"
			record.Success = draft.Pricing != nil && strings.TrimSpace(draft.Pricing.Currency) != "" && (draft.Pricing.Currency != before || before == "")
		case "missing_sku", "invalid_sku":
			before := firstVariantSKU(draft)
			f.fixVariants(draft)
			record.Action = "fill_variant_sku"
			record.Success = firstVariantSKU(draft) != "" && firstVariantSKU(draft) != before || firstVariantSKU(draft) != ""
		default:
			record.Action = "run_generic_autofix"
			f.Fix(req, draft)
			record.Success = true
		}
		history = append(history, record)
	}

	f.Fix(req, draft)
	return history
}

func (f *autoFixer) fixBrand(req *GenerateRequest, draft *AmazonListingDraft) {
	if strings.TrimSpace(draft.Brand) != "" {
		return
	}
	if req != nil && strings.TrimSpace(req.BrandHint) != "" {
		draft.Brand = strings.TrimSpace(req.BrandHint)
	}
	if draft.Brand == "" && draft.Attributes != nil {
		if brand := strings.TrimSpace(draft.Attributes["brand"]); brand != "" {
			draft.Brand = brand
		}
	}
	if draft.Brand != "" {
		if draft.Attributes == nil {
			draft.Attributes = make(map[string]string)
		}
		if _, exists := draft.Attributes["brand"]; !exists {
			draft.Attributes["brand"] = draft.Brand
		}
	}
}

func (f *autoFixer) fixTitle(draft *AmazonListingDraft) {
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		return
	}
	runes := []rune(title)
	if len(runes) > 200 {
		title = strings.TrimSpace(string(runes[:200]))
	}
	draft.Title = title
}

func (f *autoFixer) fixBulletPoints(draft *AmazonListingDraft) {
	seen := map[string]struct{}{}
	bullets := make([]string, 0, len(draft.BulletPoints))
	for _, bullet := range draft.BulletPoints {
		bullet = normalizeSentence(bullet)
		if bullet == "" {
			continue
		}
		key := strings.ToLower(bullet)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		bullets = append(bullets, bullet)
	}

	if len(bullets) < 3 {
		candidates := make([]string, 0, 8)
		candidates = append(candidates, draft.SearchTerms...)
		if draft.ProductType != "" {
			candidates = append(candidates, "Designed for "+draft.ProductType)
		}
		for key, value := range draft.Attributes {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			candidates = append(candidates, key+": "+value)
		}
		candidates = append(candidates, splitDescription(draft.Description)...)
		for _, candidate := range candidates {
			candidate = normalizeSentence(candidate)
			if candidate == "" {
				continue
			}
			key := strings.ToLower(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			bullets = append(bullets, candidate)
			if len(bullets) >= 5 {
				break
			}
		}
	}
	if len(bullets) > 5 {
		bullets = bullets[:5]
	}
	draft.BulletPoints = bullets
}

func (f *autoFixer) fixPricing(req *GenerateRequest, draft *AmazonListingDraft) {
	if draft.Pricing == nil {
		draft.Pricing = &AmazonPricingDraft{}
	}
	if strings.TrimSpace(draft.Pricing.Currency) == "" {
		country := ""
		if req != nil {
			country = req.Country
		}
		if country == "" {
			country = draft.Country
		}
		draft.Pricing.Currency = currencyByCountry(country)
	}
}

func (f *autoFixer) fixVariants(draft *AmazonListingDraft) {
	if len(draft.Variants) == 0 {
		return
	}
	defaultCount := 0
	for i := range draft.Variants {
		if strings.TrimSpace(draft.Variants[i].SKU) == "" {
			draft.Variants[i].SKU = buildFallbackSKU(draft, i)
		}
		if draft.Variants[i].Price != nil && strings.TrimSpace(draft.Variants[i].Price.Currency) == "" {
			draft.Variants[i].Price.Currency = currencyByCountry(draft.Country)
		}
		if draft.Variants[i].IsDefault {
			defaultCount++
		}
	}
	if defaultCount == 0 {
		draft.Variants[0].IsDefault = true
	}
}

func (f *autoFixer) fixImages(draft *AmazonListingDraft) {
	if draft.Images == nil {
		return
	}
	if draft.Images.MainImage == "" && len(draft.Images.RawInputImages) > 0 {
		draft.Images.MainImage = draft.Images.RawInputImages[0]
	}
}

func buildFallbackSKU(draft *AmazonListingDraft, index int) string {
	base := strings.TrimSpace(draft.Brand)
	if base == "" {
		base = strings.TrimSpace(draft.ProductType)
	}
	if base == "" {
		base = "ITEM"
	}
	base = strings.ToUpper(strings.ReplaceAll(base, " ", "-"))
	return base + "-" + strings.TrimSpace(strings.ToUpper(intToAlphaNum(index+1)))
}

func intToAlphaNum(v int) string {
	if v < 10 {
		return string(rune('0' + v))
	}
	return string(rune('A' + (v - 10)))
}

func splitDescription(description string) []string {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}
	parts := strings.FieldsFunc(description, func(r rune) bool {
		return r == '.' || r == ';' || r == '\n'
	})
	results := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeSentence(part)
		if part == "" {
			continue
		}
		results = append(results, part)
	}
	return results
}

func normalizeSentence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "-• ")
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) > 250 {
		s = strings.TrimSpace(string(runes[:250]))
	}
	return s
}

func firstVariantSKU(draft *AmazonListingDraft) string {
	for _, variant := range draft.Variants {
		if strings.TrimSpace(variant.SKU) != "" {
			return variant.SKU
		}
	}
	return ""
}
