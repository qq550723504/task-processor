package shein

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

func translateSubmitLocalizedList(items []sheinproduct.LanguageContent, fallback string, targetLanguages []string, api sheintranslateapi.TranslateAPI) ([]sheinproduct.LanguageContent, error) {
	sourceText := firstSubmitLocalizedText(items)
	if sourceText == "" {
		sourceText = strings.TrimSpace(fallback)
	}
	if sourceText == "" {
		return compactSubmitLocalizedList(items), nil
	}
	sourceLang := detectSubmitLanguage(sourceText, "")

	byLanguage := make(map[string]string, len(items)+len(targetLanguages))
	for _, item := range items {
		lang := normalizeSubmitLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" {
			continue
		}
		byLanguage[lang] = text
	}

	for _, target := range targetLanguages {
		target = normalizeSubmitLanguage(target)
		if target == "" {
			continue
		}
		existing := strings.TrimSpace(byLanguage[target])
		if existing != "" && !submitTextNeedsTranslation(existing, target) {
			continue
		}
		if target == sourceLang {
			byLanguage[target] = sourceText
			continue
		}
		if api == nil {
			byLanguage[target] = sourceText
			continue
		}
		translated, err := api.Translate(sourceText, sourceLang, target)
		if err != nil {
			return nil, err
		}
		if translated = strings.TrimSpace(translated); translated != "" {
			byLanguage[target] = translated
		}
	}

	result := make([]sheinproduct.LanguageContent, 0, len(byLanguage))
	seen := map[string]bool{}
	for _, target := range targetLanguages {
		target = normalizeSubmitLanguage(target)
		if target == "" || seen[target] {
			continue
		}
		if text := strings.TrimSpace(byLanguage[target]); text != "" {
			result = append(result, sheinproduct.LanguageContent{Language: target, Name: text})
			seen[target] = true
		}
	}
	for _, item := range items {
		lang := normalizeSubmitLanguage(item.Language)
		if lang == "" || seen[lang] {
			continue
		}
		if text := strings.TrimSpace(item.Name); text != "" {
			result = append(result, sheinproduct.LanguageContent{Language: lang, Name: text})
			seen[lang] = true
		}
	}
	return result, nil
}

func firstSubmitLocalizedText(items []sheinproduct.LanguageContent) string {
	for _, item := range items {
		if text := strings.TrimSpace(item.Name); text != "" {
			return text
		}
	}
	return ""
}

func compactSubmitLocalizedList(items []sheinproduct.LanguageContent) []sheinproduct.LanguageContent {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.LanguageContent, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		lang := normalizeSubmitLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" || seen[lang] {
			continue
		}
		result = append(result, sheinproduct.LanguageContent{Language: lang, Name: text})
		seen[lang] = true
	}
	return result
}

func submitLocalizedListNeedsTranslation(items []sheinproduct.LanguageContent) bool {
	for _, item := range items {
		if submitTextNeedsTranslation(item.Name, item.Language) {
			return true
		}
	}
	return false
}

func submitLocalizedListMissingTargets(items []sheinproduct.LanguageContent, targetLanguages []string) bool {
	if len(targetLanguages) == 0 {
		return false
	}
	byLanguage := make(map[string]string, len(items))
	for _, item := range items {
		lang := normalizeSubmitLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" {
			continue
		}
		byLanguage[lang] = text
	}
	for _, target := range targetLanguages {
		target = normalizeSubmitLanguage(target)
		if target == "" {
			continue
		}
		if strings.TrimSpace(byLanguage[target]) == "" {
			return true
		}
	}
	return false
}

func submitTextNeedsTranslation(text string, language string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lang := normalizeSubmitLanguage(language)
	return submitContainsCJK(text) && lang != "zh" && lang != "ja"
}

func findSubmitLanguageContent(items []sheinproduct.LanguageContent, language string) string {
	language = normalizeSubmitLanguage(language)
	for _, item := range items {
		if normalizeSubmitLanguage(item.Language) == language {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}

func normalizeSubmitLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return "en"
	}
	if index := strings.IndexAny(language, "-_"); index > 0 {
		language = language[:index]
	}
	return language
}

func detectSubmitLanguage(title, description string) string {
	text := strings.TrimSpace(title + " " + description)
	if text == "" {
		return "en"
	}

	var japaneseCount, chineseCount, englishCount int
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF):
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF:
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	if japaneseCount > chineseCount && japaneseCount > englishCount {
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		return "zh"
	}
	return "en"
}

func submitTargetLanguagesByRegion(region string) []string {
	switch region {
	case "US", "MX":
		return []string{"en", "es"}
	case "FR", "DE", "IT", "ES":
		return []string{"de", "es", "fr", "it", "en"}
	case "JP":
		return []string{"ja", "en"}
	case "SA", "AE":
		return []string{"ar", "en"}
	default:
		return []string{"en"}
	}
}

func submitContainsCJK(text string) bool {
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x30FF) || (r >= 0x4E00 && r <= 0x9FFF) {
			return true
		}
	}
	return false
}
