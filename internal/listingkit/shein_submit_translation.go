package listingkit

import (
	"fmt"
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
	sheintranslate "task-processor/internal/shein/translate"
)

func translateSheinProductContentForSubmit(product *sheinproduct.Product, api sheintranslateapi.TranslateAPI, region string) error {
	if product == nil {
		return nil
	}
	targetLanguages := sheintranslate.GetTargetLanguagesByRegion(strings.ToUpper(strings.TrimSpace(region)))
	if len(targetLanguages) == 0 {
		targetLanguages = []string{"en"}
	}

	var err error
	product.MultiLanguageNameList, err = translateSheinLocalizedList(product.MultiLanguageNameList, "", targetLanguages, api)
	if err != nil {
		return fmt.Errorf("translate SHEIN product name: %w", err)
	}
	product.MultiLanguageDescList, err = translateSheinLocalizedList(product.MultiLanguageDescList, "", targetLanguages, api)
	if err != nil {
		return fmt.Errorf("translate SHEIN product description: %w", err)
	}
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		fallback := strings.TrimSpace(skc.MultiLanguageName.Name)
		skc.MultiLanguageNameList, err = translateSheinLocalizedList(skc.MultiLanguageNameList, fallback, targetLanguages, api)
		if err != nil {
			return fmt.Errorf("translate SHEIN SKC name: %w", err)
		}
		if translated := findSheinLanguageContent(skc.MultiLanguageNameList, "en"); translated != "" {
			skc.MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: translated}
		}
	}
	return nil
}

func sheinProductNeedsContentTranslation(product *sheinproduct.Product) bool {
	if product == nil {
		return false
	}
	if sheinLocalizedListNeedsTranslation(product.MultiLanguageNameList) || sheinLocalizedListNeedsTranslation(product.MultiLanguageDescList) {
		return true
	}
	for _, skc := range product.SKCList {
		if sheinLocalizedListNeedsTranslation(skc.MultiLanguageNameList) || sheinTextNeedsTranslation(skc.MultiLanguageName.Name, skc.MultiLanguageName.Language) {
			return true
		}
	}
	return false
}

func translateSheinLocalizedList(items []sheinproduct.LanguageContent, fallback string, targetLanguages []string, api sheintranslateapi.TranslateAPI) ([]sheinproduct.LanguageContent, error) {
	sourceText := firstSheinLocalizedText(items)
	if sourceText == "" {
		sourceText = strings.TrimSpace(fallback)
	}
	if sourceText == "" {
		return compactSheinLocalizedList(items), nil
	}
	sourceLang := detectSheinSubmitLanguage(sourceText)

	byLanguage := make(map[string]string, len(items)+len(targetLanguages))
	for _, item := range items {
		lang := normalizeSheinLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" {
			continue
		}
		byLanguage[lang] = text
	}

	for _, target := range targetLanguages {
		target = normalizeSheinLanguage(target)
		if target == "" {
			continue
		}
		existing := strings.TrimSpace(byLanguage[target])
		if existing != "" && !sheinTextNeedsTranslation(existing, target) {
			continue
		}
		if target == sourceLang {
			byLanguage[target] = sourceText
			continue
		}
		if api == nil {
			if sheinTextNeedsTranslation(sourceText, target) || existing != "" {
				return nil, fmt.Errorf("translate API is not configured for target language %s", target)
			}
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
		target = normalizeSheinLanguage(target)
		if target == "" || seen[target] {
			continue
		}
		if text := strings.TrimSpace(byLanguage[target]); text != "" {
			result = append(result, sheinproduct.LanguageContent{Language: target, Name: text})
			seen[target] = true
		}
	}
	for _, item := range items {
		lang := normalizeSheinLanguage(item.Language)
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

func compactSheinLocalizedList(items []sheinproduct.LanguageContent) []sheinproduct.LanguageContent {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.LanguageContent, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		lang := normalizeSheinLanguage(item.Language)
		text := strings.TrimSpace(item.Name)
		if lang == "" || text == "" || seen[lang] {
			continue
		}
		result = append(result, sheinproduct.LanguageContent{Language: lang, Name: text})
		seen[lang] = true
	}
	return result
}

func sheinLocalizedListNeedsTranslation(items []sheinproduct.LanguageContent) bool {
	for _, item := range items {
		if sheinTextNeedsTranslation(item.Name, item.Language) {
			return true
		}
	}
	return false
}

func sheinTextNeedsTranslation(text string, language string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lang := normalizeSheinLanguage(language)
	return containsSheinCJK(text) && lang != "zh" && lang != "ja"
}

func firstSheinLocalizedText(items []sheinproduct.LanguageContent) string {
	for _, item := range items {
		if text := strings.TrimSpace(item.Name); text != "" {
			return text
		}
	}
	return ""
}

func findSheinLanguageContent(items []sheinproduct.LanguageContent, language string) string {
	language = normalizeSheinLanguage(language)
	for _, item := range items {
		if normalizeSheinLanguage(item.Language) == language {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}

func detectSheinSubmitLanguage(text string) string {
	var japaneseCount, chineseCount, englishCount int
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x30FF):
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

func containsSheinCJK(text string) bool {
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x30FF) || (r >= 0x4E00 && r <= 0x9FFF) {
			return true
		}
	}
	return false
}

func normalizeSheinLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return "en"
	}
	if index := strings.IndexAny(language, "-_"); index > 0 {
		language = language[:index]
	}
	return language
}
