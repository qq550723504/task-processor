package shein

import (
	"regexp"
	"strings"
	"unicode"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type listingCopy struct {
	Title            string
	Description      string
	SKCTitleBase     string
	TitleDiagnostics *TitleDiagnostics
}

func buildSheinListingCopy(canonical *canonical.Product, fallbackTitle string, aiClient openaiclient.ChatCompleter) listingCopy {
	titleResolution := resolveListingTitle(canonical, fallbackTitle, aiClient)
	titleResolution = enrichResolvedListingTitle(titleResolution, canonical, fallbackTitle, aiClient)
	title := titleResolution.title
	description := firstEnglishCandidate(canonicalDescription(canonical))
	if description == "" || containsCJK(description) {
		description = synthesizeEnglishDescription(canonical, title)
	}
	return listingCopy{
		Title:        cleanListingText(title),
		Description:  cleanListingText(description),
		SKCTitleBase: titleResolution.skcBase,
		TitleDiagnostics: &TitleDiagnostics{
			Source:             titleResolution.source,
			PromptContaminated: titleResolution.contaminate,
			ResolutionNote:     titleResolution.note,
			SKCBaseTitle:       titleResolution.skcBase,
		},
	}
}

func NormalizeListingCopy(pkg *Package, canonical *canonical.Product, language string) bool {
	NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	copy := buildSheinListingCopy(canonical, firstNonEmpty(pkg.ProductNameEn, pkg.SpuName), nil)
	changed := false
	if copy.Title != "" && (strings.TrimSpace(pkg.ProductNameEn) == "" || containsCJK(pkg.ProductNameEn)) {
		pkg.ProductNameEn = copy.Title
		pkg.ProductNameMulti = copy.Title
		changed = true
	}
	if copy.Description != "" && (strings.TrimSpace(pkg.Description) == "" || containsCJK(pkg.Description)) {
		pkg.Description = copy.Description
		changed = true
	}
	if pkg.DraftPayload != nil {
		if copy.Title != "" && localizedTextsContainCJK(pkg.DraftPayload.MultiLanguageNameList) {
			pkg.DraftPayload.MultiLanguageNameList = localizedEnglishText(language, copy.Title)
			changed = true
		}
		if copy.Description != "" && localizedTextsContainCJK(pkg.DraftPayload.MultiLanguageDescList) {
			pkg.DraftPayload.MultiLanguageDescList = localizedEnglishText(language, copy.Description)
			changed = true
		}
	}
	return changed
}

func firstEnglishCandidate(values ...string) string {
	for _, value := range values {
		value = cleanListingText(value)
		if value == "" || containsCJK(value) {
			continue
		}
		return value
	}
	return ""
}

func synthesizeEnglishTitle(canonical *canonical.Product, fallbackTitle string) string {
	productType := inferEnglishProductType(canonical, fallbackTitle)
	style := lookupVariantAttribute(canonical, "ai_style")
	size := firstNonEmpty(
		lookupVariantAttribute(canonical, "Size"),
		lookupVariantAttribute(canonical, "size"),
		lookupTechnicalSpec(canonical, "size"),
	)
	material := normalizeEnglishMaterial(firstNonEmpty(
		lookupCanonicalAttribute(canonical, "material"),
		lookupTechnicalSpec(canonical, "material"),
	))
	parts := []string{"Custom"}
	if style != "" &&
		!containsCJK(style) &&
		!strings.HasPrefix(normalizeText(style), "style ") &&
		!isPromptLikeTitle(style) &&
		!isPromptLikeSaleAttributeValue(style) {
		parts = append(parts, style)
	}
	if material != "" {
		parts = append(parts, material)
	}
	parts = append(parts, productType)
	if size != "" && !containsCJK(size) {
		parts = append(parts, normalizeSizeText(size))
	}
	return strings.Join(uniqueNonEmpty(parts), " ")
}

func synthesizeEnglishDescription(canonical *canonical.Product, title string) string {
	productType := inferEnglishProductType(canonical, title)
	material := normalizeEnglishMaterial(firstNonEmpty(
		lookupCanonicalAttribute(canonical, "material"),
		lookupTechnicalSpec(canonical, "material"),
	))
	if material == "" {
		material = "durable material"
	}
	size := firstNonEmpty(
		lookupVariantAttribute(canonical, "Size"),
		lookupTechnicalSpec(canonical, "size"),
	)
	var sentences []string
	sentences = append(sentences, title+" designed for everyday home decor and gift-ready selling.")
	sentences = append(sentences, "Made with "+material+" and finished for reliable daily use.")
	if size != "" && !containsCJK(size) {
		sentences = append(sentences, "Size: "+normalizeSizeText(size)+".")
	}
	sentences = append(sentences, "Suitable for bedrooms, living rooms, offices, dorm rooms and seasonal decor.")
	if productType != "" {
		sentences = append(sentences, "Product type: "+productType+".")
	}
	return strings.Join(sentences, " ")
}

func inferEnglishProductType(canonical *canonical.Product, fallback string) string {
	signals := []string{
		lookupCanonicalAttribute(canonical, "product_english_name"),
		lookupCanonicalAttribute(canonical, "english_name"),
		fallback,
	}
	if canonical != nil {
		signals = append(signals, canonical.CategoryPath...)
	}
	for _, signal := range signals {
		value := cleanListingText(signal)
		if value == "" || containsCJK(value) || isPromptLikeTitle(value) {
			continue
		}
		return titleCaseWords(value)
	}
	fallback = cleanListingText(fallback)
	if fallback == "" || containsCJK(fallback) || isPromptLikeTitle(fallback) {
		return ""
	}
	return titleCaseWords(fallback)
}

func normalizeEnglishMaterial(value string) string {
	value = cleanListingText(value)
	if value == "" {
		return ""
	}
	normalized := normalizeText(value)
	switch {
	case strings.Contains(normalized, "wood") || strings.Contains(normalized, "木"):
		return "wood"
	case strings.Contains(normalized, "polyester") || strings.Contains(normalized, "涤纶"):
		return "polyester"
	case strings.Contains(normalized, "cotton") || strings.Contains(normalized, "棉"):
		return "cotton"
	case containsCJK(value):
		return ""
	default:
		return value
	}
}

func lookupCanonicalAttribute(canonical *canonical.Product, key string) string {
	if canonical == nil || len(canonical.Attributes) == 0 {
		return ""
	}
	key = normalizeText(key)
	for name, value := range canonical.Attributes {
		if normalizeText(name) == key {
			return value.Value
		}
	}
	return ""
}

func lookupVariantAttribute(canonical *canonical.Product, key string) string {
	if canonical == nil || len(canonical.Variants) == 0 {
		return ""
	}
	key = normalizeText(key)
	for _, variant := range canonical.Variants {
		for name, value := range variant.Attributes {
			if normalizeText(name) == key && strings.TrimSpace(value.Value) != "" {
				return value.Value
			}
		}
	}
	return ""
}

func lookupTechnicalSpec(canonical *canonical.Product, key string) string {
	if canonical == nil || canonical.Specifications == nil || len(canonical.Specifications.Technical) == 0 {
		return ""
	}
	key = normalizeText(key)
	for name, value := range canonical.Specifications.Technical {
		if normalizeText(name) == key {
			return value
		}
	}
	return ""
}

func canonicalDescription(canonical *canonical.Product) string {
	if canonical == nil {
		return ""
	}
	return canonical.Description
}

func normalizeSizeText(value string) string {
	value = cleanListingText(value)
	value = strings.ReplaceAll(value, "×", " x ")
	value = strings.ReplaceAll(value, "*", " x ")
	return strings.Join(strings.Fields(value), " ")
}

func cleanListingText(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	return value
}

func titleCaseWords(value string) string {
	words := strings.Fields(cleanListingText(value))
	for i, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func containsCJK(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = cleanListingText(value)
		if value == "" {
			continue
		}
		key := normalizeText(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

var listingUnsafeChars = regexp.MustCompile(`[^\p{L}\p{N}\s.,'"/+\-xX()]`)

func sanitizeListingCopy(value string) string {
	return strings.TrimSpace(listingUnsafeChars.ReplaceAllString(value, " "))
}

func localizedEnglishText(language string, value string) []LocalizedText {
	value = sanitizeListingCopy(cleanListingText(value))
	language = strings.TrimSpace(language)
	if language == "" {
		language = "en_US"
	}
	return []LocalizedText{
		{Language: language, Name: value},
		{Language: "en", Name: value},
	}
}

func localizedTextsContainCJK(items []LocalizedText) bool {
	if len(items) == 0 {
		return true
	}
	for _, item := range items {
		if containsCJK(item.Name) {
			return true
		}
	}
	return false
}
