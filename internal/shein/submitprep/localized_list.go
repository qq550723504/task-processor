package submitprep

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

func TranslateLocalizedList(items []sheinproduct.LanguageContent, fallback string, targetLanguages []string, api sheintranslateapi.TranslateAPI) ([]sheinproduct.LanguageContent, error) {
	sourceText := FirstLocalizedText(items)
	if sourceText == "" {
		sourceText = strings.TrimSpace(fallback)
	}
	if sourceText == "" {
		return CompactLocalizedList(items), nil
	}
	sourceLang := DetectLanguage(sourceText, "")

	byLanguage := make(map[string]string, len(items)+len(targetLanguages))
	for _, item := range items {
		lang := NormalizeLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" {
			continue
		}
		byLanguage[lang] = text
	}

	for _, target := range targetLanguages {
		target = NormalizeLanguage(target)
		if target == "" {
			continue
		}
		existing := strings.TrimSpace(byLanguage[target])
		if existing != "" && !TextNeedsTranslation(existing, target) {
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
		target = NormalizeLanguage(target)
		if target == "" || seen[target] {
			continue
		}
		if text := strings.TrimSpace(byLanguage[target]); text != "" {
			result = append(result, sheinproduct.LanguageContent{Language: target, Name: text})
			seen[target] = true
		}
	}
	for _, item := range items {
		lang := NormalizeLanguage(item.Language)
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

func FirstLocalizedText(items []sheinproduct.LanguageContent) string {
	for _, item := range items {
		if text := strings.TrimSpace(item.Name); text != "" {
			return text
		}
	}
	return ""
}

func CompactLocalizedList(items []sheinproduct.LanguageContent) []sheinproduct.LanguageContent {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.LanguageContent, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		lang := NormalizeLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" || seen[lang] {
			continue
		}
		result = append(result, sheinproduct.LanguageContent{Language: lang, Name: text})
		seen[lang] = true
	}
	return result
}

func LocalizedListNeedsTranslation(items []sheinproduct.LanguageContent) bool {
	for _, item := range items {
		if TextNeedsTranslation(item.Name, item.Language) {
			return true
		}
	}
	return false
}

func LocalizedListMissingTargets(items []sheinproduct.LanguageContent, targetLanguages []string) bool {
	if len(targetLanguages) == 0 {
		return false
	}
	byLanguage := make(map[string]string, len(items))
	for _, item := range items {
		lang := NormalizeLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" {
			continue
		}
		byLanguage[lang] = text
	}
	for _, target := range targetLanguages {
		target = NormalizeLanguage(target)
		if target == "" {
			continue
		}
		if strings.TrimSpace(byLanguage[target]) == "" {
			return true
		}
	}
	return false
}

func TextNeedsTranslation(text string, language string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lang := NormalizeLanguage(language)
	return containsCJK(text) && lang != "zh" && lang != "ja"
}

func FindLanguageContent(items []sheinproduct.LanguageContent, language string) string {
	language = NormalizeLanguage(language)
	for _, item := range items {
		if NormalizeLanguage(item.Language) == language {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}

func NormalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return "en"
	}
	if index := strings.IndexAny(language, "-_"); index > 0 {
		language = language[:index]
	}
	return language
}

func containsCJK(text string) bool {
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x30FF) || (r >= 0x4E00 && r <= 0x9FFF) {
			return true
		}
	}
	return false
}
