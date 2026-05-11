package submitprep

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/shein/aicache"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslateapi "task-processor/internal/shein/api/translate"
	"task-processor/internal/shein/content"
)

func BuildLocalizedTitleAndDescription(ctx context.Context, region string, title string, description string, features string, brand string, aiClient openaiclient.ChatCompleter, cache *aicache.Cache, translateAPI sheintranslateapi.TranslateAPI) ([]sheinproduct.LanguageContent, []sheinproduct.LanguageContent, error) {
	cleaner := content.NewTextCleaner()
	optimizer := content.NewContentOptimizer(aiClient)

	cleanedTitle := cleaner.RemoveBrandFromText(title, brand)
	cleanedDescription := cleaner.RemoveBrandFromText(description, brand)
	detectedLang := DetectLanguage(cleanedTitle, cleanedDescription)

	optimizedTitle := cleanedTitle
	optimizedDescription := cleanedDescription
	if detectedLang == "en" {
		aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
		defer cancel()
		titleOut, descriptionOut, err := optimizer.OptimizeTitleAndDescriptionWithCache(aiCtx, cleanedTitle, cleanedDescription, features, cache)
		if err == nil {
			optimizedTitle = titleOut
			optimizedDescription = descriptionOut
		}
	}

	optimizedTitle = ensureValidText(optimizedTitle, cleanedTitle, title, "Quality Product")
	optimizedDescription = ensureValidText(optimizedDescription, cleanedDescription, description, "High quality product with excellent features and design.")

	targetLanguages := GetTargetLanguagesByRegion(strings.ToUpper(strings.TrimSpace(region)))
	if len(targetLanguages) == 0 {
		return nil, nil, fmt.Errorf("unsupported region: %s", region)
	}

	nameList, err := buildLocalizedList(optimizedTitle, detectedLang, targetLanguages, translateAPI, false)
	if err != nil {
		return nil, nil, fmt.Errorf("translate product title: %w", err)
	}

	descSource := optimizedDescription
	descList, err := buildLocalizedList(truncateDescription(descSource), detectedLang, targetLanguages, translateAPI, true)
	if err != nil {
		fallbackDescription := strings.TrimSpace(features)
		if fallbackDescription == "" {
			fallbackDescription = "High quality product with excellent features and design."
		}
		descList, err = buildLocalizedList(truncateDescription(fallbackDescription), detectedLang, targetLanguages, translateAPI, true)
		if err != nil {
			return nil, nil, fmt.Errorf("translate product description: %w", err)
		}
	}

	return nameList, descList, nil
}

func buildLocalizedList(sourceText string, sourceLang string, targetLanguages []string, translateAPI sheintranslateapi.TranslateAPI, truncateDesc bool) ([]sheinproduct.LanguageContent, error) {
	sourceText = strings.TrimSpace(sourceText)
	if sourceText == "" {
		return nil, nil
	}
	list := make([]sheinproduct.LanguageContent, 0, len(targetLanguages))
	for _, targetLang := range targetLanguages {
		text := sourceText
		if targetLang != sourceLang {
			if translateAPI != nil {
				translated, err := translateAPI.Translate(sourceText, sourceLang, targetLang)
				if err != nil {
					return nil, err
				}
				text = translated
			}
		}
		if truncateDesc {
			text = truncateDescription(text)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		list = appendIfLanguageMissing(list, targetLang, text)
	}
	return list, nil
}

func appendIfLanguageMissing(items []sheinproduct.LanguageContent, language string, text string) []sheinproduct.LanguageContent {
	for _, item := range items {
		if item.Language == language {
			return items
		}
	}
	return append(items, sheinproduct.LanguageContent{Language: language, Name: text})
}

func ensureValidText(primary string, secondary string, original string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	if strings.TrimSpace(secondary) != "" {
		return strings.TrimSpace(secondary)
	}
	if strings.TrimSpace(original) != "" {
		return strings.TrimSpace(original)
	}
	return fallback
}

func truncateDescription(description string) string {
	const maxDescriptionLength = 5000
	if len(description) <= maxDescriptionLength {
		return description
	}
	truncated := description[:maxDescriptionLength]
	lastPeriod := strings.LastIndexAny(truncated, ".!?")
	if lastPeriod > 0 && lastPeriod > maxDescriptionLength-200 {
		truncated = truncated[:lastPeriod+1]
	}
	return strings.TrimSpace(truncated)
}
