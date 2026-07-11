package referenceanalysis

import (
	"regexp"
	"strings"
)

func abstractStudioReferenceAnalyses(analyses []imageAnalysis) []abstractedAnalysis {
	result := make([]abstractedAnalysis, 0, len(analyses))
	for _, item := range analyses {
		abstracted := abstractStudioReferenceAnalysis(item)
		if abstracted.Motif == "" &&
			len(abstracted.Palette) == 0 &&
			len(abstracted.Composition) == 0 &&
			abstracted.Typography == "" &&
			abstracted.Density == "" &&
			abstracted.ProductFit == "" &&
			abstracted.Mood == "" &&
			abstracted.GarmentPlacement == "" &&
			!(abstracted.HadUnsafe || abstracted.HadMalformed) {
			continue
		}
		result = append(result, abstracted)
	}
	return result
}

func hasStudioReusableSafeStyleDirection(analyses []abstractedAnalysis) bool {
	for _, item := range analyses {
		if abstractedAnalysisHasReusableSignals(item) {
			return true
		}
	}
	return false
}

func abstractedAnalysisHasReusableSignals(item abstractedAnalysis) bool {
	return item.Motif != "" ||
		len(item.Palette) > 0 ||
		len(item.Composition) > 0 ||
		item.Typography != "" ||
		item.Density != "" ||
		item.ProductFit != "" ||
		item.Mood != "" ||
		item.GarmentPlacement != ""
}

func abstractStudioReferenceAnalysis(item imageAnalysis) abstractedAnalysis {
	abstracted := abstractedAnalysis{
		Motif:            abstractStudioMotif(item.Motif),
		Palette:          abstractStudioPalette(item.Palette),
		Composition:      abstractStudioComposition(item.Composition),
		Typography:       abstractStudioTypography(item.Typography),
		Density:          abstractStudioDensity(item.Density),
		ProductFit:       abstractStudioProductFit(item.ProductFit),
		Mood:             abstractStudioMood(item.Mood),
		GarmentPlacement: abstractStudioGarmentPlacement(item.GarmentPlacement),
	}
	abstracted.HadMalformed = isMalformedStudioReferenceAnalysis(item)

	if abstracted.HadMalformed {
		rawUnsafe := studioReferenceFieldContainsUnsafeSignals(item.Raw)
		if !rawUnsafe && abstracted.Motif == "" {
			abstracted.Motif = abstractStudioMotif(item.Raw)
		}
		if !rawUnsafe && len(abstracted.Palette) == 0 {
			abstracted.Palette = abstractStudioPalette([]string{item.Raw})
		}
		if len(abstracted.Composition) == 0 {
			abstracted.Composition = abstractStudioComposition(item.Raw)
		}
		if !rawUnsafe && abstracted.Typography == "" {
			abstracted.Typography = recoverMalformedStudioReferenceField(item.Raw, studioTypographyPhraseVocabulary, studioTypographyWordVocabulary, abstractStudioTypography)
		}
		if !rawUnsafe && abstracted.Density == "" {
			abstracted.Density = recoverMalformedStudioReferenceField(item.Raw, studioDensityPhraseVocabulary, studioDensityWordVocabulary, abstractStudioDensity)
		}
		if !rawUnsafe && abstracted.ProductFit == "" {
			abstracted.ProductFit = recoverMalformedStudioReferenceField(item.Raw, studioProductFitPhraseVocabulary, studioProductFitWordVocabulary, abstractStudioProductFit)
		}
		if !rawUnsafe && abstracted.Mood == "" {
			abstracted.Mood = abstractStudioMood(item.Raw)
		}
		if !rawUnsafe && abstracted.GarmentPlacement == "" {
			abstracted.GarmentPlacement = abstractStudioGarmentPlacement(item.Raw)
		}
		if !rawUnsafe && !abstractedAnalysisHasReusableSignals(abstracted) {
			abstracted = mergeStudioAbstractedReferenceAnalysis(abstracted, deriveConservativeStudioMalformedFallback(item.Raw))
		}
	}

	applyStructuredStudioFallbacks(item, &abstracted)
	abstracted.HadUnsafe = studioReferenceAnalysisContainsUnsafeSignals(item)
	for _, avoid := range item.Avoid {
		if studioReferenceAvoidContainsUnsafeSignals(avoid) {
			abstracted.HadUnsafe = true
		}
	}
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Motif, abstracted.Motif)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Typography, abstracted.Typography)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Density, abstracted.Density)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.ProductFit, abstracted.ProductFit)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.Mood, abstracted.Mood)
	abstracted.HadUnsafe = abstracted.HadUnsafe || studioStructuredFieldDroppedDueToUnsafeSignal(item.GarmentPlacement, abstracted.GarmentPlacement)
	return abstracted
}

func mergeStudioAbstractedReferenceAnalysis(primary abstractedAnalysis, fallback abstractedAnalysis) abstractedAnalysis {
	if primary.Motif == "" {
		primary.Motif = fallback.Motif
	}
	if len(primary.Palette) == 0 {
		primary.Palette = fallback.Palette
	}
	if len(primary.Composition) == 0 {
		primary.Composition = fallback.Composition
	}
	if primary.Typography == "" {
		primary.Typography = fallback.Typography
	}
	if primary.Density == "" {
		primary.Density = fallback.Density
	}
	if primary.ProductFit == "" {
		primary.ProductFit = fallback.ProductFit
	}
	if primary.Mood == "" {
		primary.Mood = fallback.Mood
	}
	if primary.GarmentPlacement == "" {
		primary.GarmentPlacement = fallback.GarmentPlacement
	}
	return primary
}

func deriveConservativeStudioMalformedFallback(raw string) abstractedAnalysis {
	if sanitizeStudioMalformedFallbackCandidate(raw) == "" {
		return abstractedAnalysis{}
	}
	return abstractedAnalysis{
		Motif:       "abstract",
		Composition: []string{"balanced composition"},
	}
}

func applyStructuredStudioFallbacks(item imageAnalysis, abstracted *abstractedAnalysis) {
	if abstracted == nil {
		return
	}
	if abstracted.Motif == "" && studioStructuredFieldCanUseFallback(item.Motif) {
		abstracted.Motif = "abstract motif direction"
	}
	if len(abstracted.Palette) == 0 && studioStructuredPaletteCanUseFallback(item.Palette) {
		abstracted.Palette = []string{"balanced palette direction"}
	}
	if abstracted.Typography == "" && studioStructuredFieldCanUseFallback(item.Typography) {
		abstracted.Typography = "decorative typography direction"
	}
	if abstracted.Density == "" && studioStructuredFieldCanUseFallback(item.Density) {
		abstracted.Density = "balanced visual density"
	}
	if abstracted.ProductFit == "" && studioStructuredFieldCanUseFallback(item.ProductFit) {
		abstracted.ProductFit = "general apparel styling"
	}
	if abstracted.Mood == "" && studioStructuredFieldCanUseFallback(item.Mood) {
		abstracted.Mood = "balanced mood"
	}
	if abstracted.GarmentPlacement == "" && studioStructuredFieldCanUseFallback(item.GarmentPlacement) {
		abstracted.GarmentPlacement = "standard garment placement"
	}
}

func studioStructuredFieldCanUseFallback(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && !studioReferenceFieldContainsUnsafeSignals(trimmed)
}

func studioStructuredPaletteCanUseFallback(values []string) bool {
	if len(values) == 0 {
		return false
	}
	hasNonEmpty := false
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		hasNonEmpty = true
		if studioReferenceFieldContainsUnsafeSignals(trimmed) {
			return false
		}
	}
	return hasNonEmpty
}

func abstractStudioMotif(value string) string {
	return abstractStudioFieldDescriptor(value, studioMotifPhraseVocabulary, studioMotifWordVocabulary)
}

func abstractStudioPalette(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		abstracted := abstractStudioFieldDescriptor(value, studioPalettePhraseVocabulary, studioPaletteWordVocabulary)
		if abstracted == "" {
			continue
		}
		if _, ok := seen[abstracted]; ok {
			continue
		}
		seen[abstracted] = struct{}{}
		result = append(result, abstracted)
	}
	return result
}

func abstractStudioTypography(value string) string {
	return abstractStudioFieldDescriptor(value, studioTypographyPhraseVocabulary, studioTypographyWordVocabulary)
}

func abstractStudioDensity(value string) string {
	return abstractStudioFieldDescriptor(value, studioDensityPhraseVocabulary, studioDensityWordVocabulary)
}

func abstractStudioProductFit(value string) string {
	return abstractStudioFieldDescriptor(value, studioProductFitPhraseVocabulary, studioProductFitWordVocabulary)
}

func abstractStudioMood(value string) string {
	lower := strings.ToLower(sanitizeStudioDescriptorCandidate(value))
	switch {
	case strings.Contains(lower, "playful"):
		return "playful mood"
	case strings.Contains(lower, "airy"):
		return "airy mood"
	case strings.Contains(lower, "nostalgic"), strings.Contains(lower, "retro"), strings.Contains(lower, "vintage"):
		return "nostalgic mood"
	case strings.Contains(lower, "coastal"), strings.Contains(lower, "resort"), strings.Contains(lower, "beach"):
		return "relaxed mood"
	default:
		return ""
	}
}

func abstractStudioGarmentPlacement(value string) string {
	lower := strings.ToLower(sanitizeStudioDescriptorCandidate(value))
	switch {
	case strings.Contains(lower, "left") && strings.Contains(lower, "chest"):
		return "left chest placement"
	case strings.Contains(lower, "right") && strings.Contains(lower, "chest"):
		return "right chest placement"
	case strings.Contains(lower, "front") && strings.Contains(lower, "chest"):
		return "front chest placement"
	case strings.Contains(lower, "large") && (strings.Contains(lower, "center") || strings.Contains(lower, "front")):
		return "large center placement"
	case strings.Contains(lower, "center"):
		return "center placement"
	case strings.Contains(lower, "back"):
		return "back placement"
	case strings.Contains(lower, "sleeve"):
		return "sleeve placement"
	default:
		return ""
	}
}

func abstractStudioComposition(value string) []string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return nil
	}
	result := make([]string, 0, 3)
	add := func(label string) {
		for _, existing := range result {
			if existing == label {
				return
			}
		}
		result = append(result, label)
	}
	if strings.Contains(lower, "center") {
		add("centered composition")
	}
	if strings.Contains(lower, "badge") || strings.Contains(lower, "crest") || strings.Contains(lower, "seal") || strings.Contains(lower, "medallion") {
		add("badge composition")
	}
	if strings.Contains(lower, "frame") || strings.Contains(lower, "framed") || strings.Contains(lower, "arch") {
		add("framed composition")
	}
	if strings.Contains(lower, "border") {
		add("border composition")
	}
	if strings.Contains(lower, "repeat") || strings.Contains(lower, "pattern") || strings.Contains(lower, "allover") {
		add("repeat pattern composition")
	}
	if strings.Contains(lower, "split") || strings.Contains(lower, "diagonal") || strings.Contains(lower, "collage") || strings.Contains(lower, "offset") || strings.Contains(lower, "layered") {
		add("balanced dynamic composition")
	}
	return result
}

func studioCompositionShouldSuppressLiteral(lower string) bool {
	for _, token := range []string{"same", "exact", "identical", "signature", "unique", "distinctive", "split", "diagonal", "collage", "offset"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func sanitizeStudioStylePhrase(value string) string {
	return sanitizeStudioSafeDescriptorPhrase(value)
}

func abstractStudioFieldDescriptor(value string, phrases []string, words map[string]string) string {
	vocabularyDerived := abstractStudioVocabularyPhrase(value, phrases, words)
	genericDerived := sanitizeStudioSafeDescriptorPhrase(value)
	return preferRicherStudioDescriptor(vocabularyDerived, genericDerived)
}

func sanitizeStudioSafeDescriptorPhrase(value string) string {
	cleaned := sanitizeStudioDescriptorCandidate(value)
	if cleaned == "" {
		return ""
	}
	tokens := studioWordPattern.FindAllString(cleaned, -1)
	if len(tokens) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := studioSafeDescriptorWords[token]; ok {
			filtered = append(filtered, token)
		}
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(filtered, " "), " "))
}

func sanitizeStudioDescriptorCandidate(value string) string {
	original := strings.TrimSpace(value)
	if original == "" {
		return ""
	}
	cleaned, _ := stripStudioProtectedIdentityPhrases(original)
	cleaned, removedSuspiciousName := stripSuspiciousStudioNamedPhrases(cleaned)
	lower := strings.ToLower(cleaned)
	lower = studioUnsafeSpacerPattern.ReplaceAllString(lower, " ")
	lower = studioProtectedIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioQuotedTextPattern.ReplaceAllString(lower, " ")
	lower = studioUniqueLayoutPattern.ReplaceAllString(lower, " ")
	lower = studioCharacterIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioExactArtworkPattern.ReplaceAllString(lower, " ")
	lower = studioExactTextPattern.ReplaceAllString(lower, " ")
	lower = studioBrandMarkPattern.ReplaceAllString(lower, " ")
	lower = studioWatermarkPattern.ReplaceAllString(lower, " ")
	lower = strings.NewReplacer("/", " ", "&", " ").Replace(lower)
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "same" || token == "exact" || token == "identical" || token == "signature" || token == "unique" || token == "distinctive" {
			continue
		}
		filtered = append(filtered, token)
	}
	if removedSuspiciousName && len(filtered) <= 1 {
		return ""
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(filtered, " "), " "))
}

func sanitizeStudioMalformedFallbackCandidate(value string) string {
	original := strings.TrimSpace(value)
	if original == "" {
		return ""
	}
	cleaned, _ := stripStudioProtectedIdentityPhrases(original)
	cleaned, _ = stripSuspiciousStudioNamedPhrases(cleaned)
	lower := strings.ToLower(cleaned)
	lower = studioUnsafeSpacerPattern.ReplaceAllString(lower, " ")
	lower = studioProtectedIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioQuotedTextPattern.ReplaceAllString(lower, " ")
	lower = studioUniqueLayoutPattern.ReplaceAllString(lower, " ")
	lower = studioCharacterIdentityPattern.ReplaceAllString(lower, " ")
	lower = studioExactArtworkPattern.ReplaceAllString(lower, " ")
	lower = studioExactTextPattern.ReplaceAllString(lower, " ")
	lower = studioBrandMarkPattern.ReplaceAllString(lower, " ")
	lower = studioWatermarkPattern.ReplaceAllString(lower, " ")
	lower = strings.NewReplacer("/", " ", "&", " ").Replace(lower)
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	return strings.TrimSpace(studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(tokens, " "), " "))
}

func stripStudioProtectedIdentityPhrases(value string) (string, bool) {
	result := value
	removed := false
	for _, term := range studioProtectedIdentityTerms {
		pattern := regexp.MustCompile(`(?i)\b` + strings.ReplaceAll(regexp.QuoteMeta(term), `\ `, `\s+`) + `\b`)
		updated := pattern.ReplaceAllString(result, " ")
		if updated != result {
			removed = true
			result = updated
		}
	}
	return result, removed
}

func stripSuspiciousStudioNamedPhrases(value string) (string, bool) {
	removed := false
	result := studioCapitalizedPhrasePattern.ReplaceAllStringFunc(value, func(match string) string {
		if studioCapitalizedPhraseIsSafe(match) {
			return match
		}
		removed = true
		return " "
	})
	return result, removed
}

func studioCapitalizedPhraseIsSafe(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	tokens := studioWordPattern.FindAllString(normalized, -1)
	if len(tokens) == 0 {
		return true
	}
	if len(tokens) > 1 {
		_, ok := studioSafeTitleCasePhraseSet[normalized]
		return ok
	}
	for _, token := range tokens {
		if _, ok := studioSafeDescriptorWords[token]; !ok {
			return false
		}
	}
	return true
}

func buildStudioSafeTitleCasePhraseSet(vocabularies ...[]string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, vocabulary := range vocabularies {
		for _, phrase := range vocabulary {
			normalized := strings.ToLower(strings.TrimSpace(phrase))
			if normalized == "" {
				continue
			}
			result[normalized] = struct{}{}
		}
	}
	return result
}

func preferRicherStudioDescriptor(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	fallback = strings.TrimSpace(fallback)
	switch {
	case primary == "":
		return fallback
	case fallback == "":
		return primary
	case primary == fallback:
		return primary
	case studioDescriptorWordCount(primary) > studioDescriptorWordCount(fallback):
		return primary
	default:
		return fallback
	}
}

func studioDescriptorWordCount(value string) int {
	return len(studioWordPattern.FindAllString(strings.ToLower(strings.TrimSpace(value)), -1))
}

func abstractStudioVocabularyPhrase(value string, phrases []string, words map[string]string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return ""
	}
	tokens := studioWordPattern.FindAllString(lower, -1)
	if len(tokens) == 0 {
		return ""
	}
	result := make([]string, 0, len(tokens))
	seen := map[string]struct{}{}
	for i := 0; i < len(tokens); {
		if i+1 < len(tokens) {
			phrase := tokens[i] + " " + tokens[i+1]
			if containsStudioVocabularyPhrase(phrases, phrase) {
				if _, ok := seen[phrase]; !ok {
					seen[phrase] = struct{}{}
					result = append(result, phrase)
				}
				i += 2
				continue
			}
		}
		if mapped, ok := words[tokens[i]]; ok {
			if _, exists := seen[mapped]; !exists {
				seen[mapped] = struct{}{}
				result = append(result, mapped)
			}
		}
		i++
	}
	return studioRepeatedWhitespacePattern.ReplaceAllString(strings.Join(result, " "), " ")
}

func recoverMalformedStudioReferenceField(raw string, phrases []string, words map[string]string, abstract func(string) string) string {
	for _, fragment := range splitMalformedStudioReferenceFragments(raw) {
		if !studioFragmentMatchesFieldVocabulary(fragment, phrases, words) {
			continue
		}
		if value := abstract(fragment); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitMalformedStudioReferenceFragments(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r':
			return true
		default:
			return false
		}
	})
	fragments := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fragments = append(fragments, trimmed)
	}
	return fragments
}

func studioFragmentMatchesFieldVocabulary(fragment string, phrases []string, words map[string]string) bool {
	lower := strings.ToLower(strings.TrimSpace(fragment))
	if lower == "" {
		return false
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, token := range studioWordPattern.FindAllString(lower, -1) {
		if _, ok := words[token]; ok {
			return true
		}
	}
	return false
}

func containsStudioVocabularyPhrase(phrases []string, target string) bool {
	for _, phrase := range phrases {
		if phrase == target {
			return true
		}
	}
	return false
}

func studioReferenceContainsUnsafeSignals(analyses []imageAnalysis, abstracted []abstractedAnalysis) bool {
	for _, item := range analyses {
		if studioReferenceAnalysisContainsUnsafeSignals(item) {
			return true
		}
		for _, avoid := range item.Avoid {
			if studioReferenceAvoidContainsUnsafeSignals(avoid) {
				return true
			}
		}
	}
	for _, item := range abstracted {
		if item.HadUnsafe {
			return true
		}
	}
	return false
}

func studioReferenceAnalysisContainsUnsafeSignals(item imageAnalysis) bool {
	if isMalformedStudioReferenceAnalysis(item) {
		return studioReferenceFieldContainsUnsafeSignals(item.Raw)
	}

	for _, value := range []string{
		item.Motif,
		item.Composition,
		item.Typography,
		item.Density,
		item.ProductFit,
		item.Mood,
		item.GarmentPlacement,
	} {
		if studioReferenceFieldContainsUnsafeSignals(value) {
			return true
		}
	}
	for _, value := range item.Palette {
		if studioReferenceFieldContainsUnsafeSignals(value) {
			return true
		}
	}
	for _, value := range item.Avoid {
		if studioReferenceAvoidContainsUnsafeSignals(value) {
			return true
		}
	}
	return false
}

func studioReferenceContainsMalformedFallback(analyses []abstractedAnalysis) bool {
	for _, item := range analyses {
		if item.HadMalformed {
			return true
		}
	}
	return false
}

func studioReferenceFieldContainsUnsafeSignals(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	return studioProtectedIdentityPattern.MatchString(value) ||
		studioQuotedTextPattern.MatchString(value) ||
		studioBrandMarkPattern.MatchString(lower) ||
		studioWatermarkPattern.MatchString(lower) ||
		studioExactArtworkPattern.MatchString(lower) ||
		studioExactTextPattern.MatchString(lower) ||
		studioCharacterIdentityPattern.MatchString(lower) ||
		studioUniqueLayoutPattern.MatchString(lower)
}

func studioReferenceAvoidContainsUnsafeSignals(value string) bool {
	return studioReferenceFieldContainsUnsafeSignals(value)
}

func studioStructuredFieldDroppedDueToUnsafeSignal(original string, abstracted string) bool {
	trimmedOriginal := strings.TrimSpace(original)
	if trimmedOriginal == "" || strings.TrimSpace(abstracted) != "" {
		return false
	}
	return studioReferenceFieldContainsUnsafeSignals(trimmedOriginal) || studioDescriptorContainsSuspiciousNamedPhrase(trimmedOriginal)
}

func studioDescriptorContainsSuspiciousNamedPhrase(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, removed := stripSuspiciousStudioNamedPhrases(value)
	return removed
}

func isMalformedStudioReferenceAnalysis(item imageAnalysis) bool {
	return item.Raw != "" &&
		item.Motif == "" &&
		len(item.Palette) == 0 &&
		item.Composition == "" &&
		item.Typography == "" &&
		item.Density == "" &&
		item.ProductFit == "" &&
		item.Mood == "" &&
		item.GarmentPlacement == "" &&
		len(item.Avoid) == 0
}
