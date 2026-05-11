package shein

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	sheincontent "task-processor/internal/shein/content"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
	sheintranslateapi "task-processor/internal/shein/api/translate"
)

func PrepareSubmitProductContent(ctx context.Context, product *sheinproduct.Product, region string, aiClient openaiclient.ChatCompleter, translateAPI sheintranslateapi.TranslateAPI) error {
	if err := optimizeSubmitProductContent(ctx, product, aiClient); err != nil {
		return err
	}
	if err := translateSubmitProductContent(product, translateAPI, region); err != nil {
		return err
	}
	if err := CleanSubmitProductSensitiveWords(product); err != nil {
		return err
	}
	return nil
}

func SubmitProductNeedsTranslation(product *sheinproduct.Product) bool {
	if product == nil {
		return false
	}
	if submitprep.LocalizedListNeedsTranslation(product.MultiLanguageNameList) || submitprep.LocalizedListNeedsTranslation(product.MultiLanguageDescList) {
		return true
	}
	for _, skc := range product.SKCList {
		if submitprep.LocalizedListNeedsTranslation(skc.MultiLanguageNameList) || submitprep.TextNeedsTranslation(skc.MultiLanguageName.Name, skc.MultiLanguageName.Language) {
			return true
		}
	}
	return false
}

func SubmitProductNeedsTargetLanguages(product *sheinproduct.Product, region string) bool {
	if product == nil {
		return false
	}
	targetLanguages := submitprep.GetTargetLanguagesByRegion(strings.ToUpper(strings.TrimSpace(region)))
	if len(targetLanguages) == 0 {
		targetLanguages = []string{"en"}
	}
	if submitprep.LocalizedListMissingTargets(product.MultiLanguageNameList, targetLanguages) || submitprep.LocalizedListMissingTargets(product.MultiLanguageDescList, targetLanguages) {
		return true
	}
	for _, skc := range product.SKCList {
		if submitprep.LocalizedListMissingTargets(skc.MultiLanguageNameList, targetLanguages) {
			return true
		}
	}
	return false
}

func CleanSubmitProductSensitiveWords(product *sheinproduct.Product) error {
	return submitprep.CleanSensitiveWords(product)
}

func RetrySensitiveWordCleanup(product *sheinproduct.Product, validationNotes []string) bool {
	return submitprep.RetrySensitiveWordCleanup(product, validationNotes)
}

func BuildSubmitSnapshot(product *sheinproduct.Product) *SubmitSnapshot {
	if product == nil {
		return nil
	}
	snapshot := &SubmitSnapshot{
		SPUName:               strings.TrimSpace(product.SPUName),
		SupplierCode:          strings.TrimSpace(product.SupplierCode),
		MultiLanguageNameList: toSubmitLocalizedTexts(product.MultiLanguageNameList),
		MultiLanguageDescList: toSubmitLocalizedTexts(product.MultiLanguageDescList),
		ImageCount:            countSubmitImages(product),
	}
	if len(product.SKCList) > 0 {
		snapshot.SKCList = make([]SubmitSKCSnapshot, 0, len(product.SKCList))
		for _, skc := range product.SKCList {
			supplierCode := ""
			if skc.SupplierCode != nil {
				supplierCode = strings.TrimSpace(*skc.SupplierCode)
			}
			snapshot.SKCList = append(snapshot.SKCList, SubmitSKCSnapshot{
				SupplierCode:          supplierCode,
				PrimaryName:           strings.TrimSpace(skc.MultiLanguageName.Name),
				MultiLanguageNameList: toSubmitLocalizedTexts(skc.MultiLanguageNameList),
			})
		}
	}
	return snapshot
}

func optimizeSubmitProductContent(ctx context.Context, product *sheinproduct.Product, aiClient openaiclient.ChatCompleter) error {
	if product == nil {
		return nil
	}
	title := submitprep.FirstLocalizedText(product.MultiLanguageNameList)
	description := submitprep.FirstLocalizedText(product.MultiLanguageDescList)
	if strings.TrimSpace(title) == "" && strings.TrimSpace(description) == "" {
		return nil
	}

	cleaner := sheincontent.NewTextCleaner()
	title = cleaner.NormalizeText(title)
	description = cleaner.NormalizeText(description)
	if title == "" {
		title = "Quality Home Decor Product"
	}
	if description == "" {
		description = title
	}
	if aiClient == nil {
		applySubmitContent(product, title, description)
		return nil
	}

	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	optimizedTitle, optimizedDescription, err := optimizeSubmitContentWithAI(aiCtx, aiClient, title, description, buildSubmitContentFeatures(product))
	if err != nil {
		applySubmitContent(product, title, description)
		return nil
	}
	optimizedTitle = strings.TrimSpace(cleaner.RemoveForbiddenWords(cleaner.NormalizeText(optimizedTitle)))
	optimizedDescription = strings.TrimSpace(cleaner.RemoveForbiddenWords(cleaner.NormalizeText(optimizedDescription)))
	if optimizedTitle == "" {
		optimizedTitle = title
	}
	if optimizedDescription == "" {
		optimizedDescription = description
	}
	applySubmitContent(product, optimizedTitle, optimizedDescription)
	return nil
}

func optimizeSubmitContentWithAI(ctx context.Context, aiClient openaiclient.ChatCompleter, title, description, features string) (string, string, error) {
	systemPrompt := `You are an e-commerce content optimization expert for SHEIN product listings.
Requirements:
1. Output title and description in natural English only.
2. Title length should be 80-800 characters when possible, concise and keyword-rich.
3. Description length should be 100-2000 characters when possible, covering material, use cases, and selling points.
4. Do not include brand names, emojis, medical claims, absolute guarantees, or platform policy-risk claims.
5. Return strict JSON only: {"title":"...","description":"..."}`
	userPrompt := fmt.Sprintf("Source product content:\nTitle: %s\nDescription: %s\nFeatures:\n%s\n\nCreate optimized SHEIN listing content.", title, description, features)
	temperature := float32(0.7)
	resp, err := aiClient.CreateChatCompletion(ctx, &openaiclient.ChatCompletionRequest{
		Model:       aiClient.GetDefaultModel(),
		Temperature: &temperature,
		Messages: []openaiclient.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return title, description, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return title, description, fmt.Errorf("AI content optimizer returned no choices")
	}
	type optimizedContent struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	var parsed optimizedContent
	clean := jsonx.CleanLLMResponse(resp.Choices[0].Message.Content)
	if err := jsonx.UnmarshalString(clean, &parsed, "parse SHEIN optimized content"); err != nil {
		return title, description, err
	}
	if strings.TrimSpace(parsed.Title) == "" {
		parsed.Title = title
	}
	if strings.TrimSpace(parsed.Description) == "" {
		parsed.Description = description
	}
	return parsed.Title, parsed.Description, nil
}

func buildSubmitContentFeatures(product *sheinproduct.Product) string {
	if product == nil {
		return ""
	}
	parts := make([]string, 0, 8)
	if product.CategoryID > 0 {
		parts = append(parts, fmt.Sprintf("SHEIN category id: %d", product.CategoryID))
	}
	for _, skc := range product.SKCList {
		if text := submitprep.FirstLocalizedText(skc.MultiLanguageNameList); text != "" {
			parts = append(parts, "Variant: "+text)
		} else if text := strings.TrimSpace(skc.MultiLanguageName.Name); text != "" {
			parts = append(parts, "Variant: "+text)
		}
		if len(parts) >= 8 {
			break
		}
	}
	return strings.Join(parts, "\n")
}

func applySubmitContent(product *sheinproduct.Product, title, description string) {
	if product == nil {
		return
	}
	product.MultiLanguageNameList = []sheinproduct.LanguageContent{{
		Language: "en",
		Name:     truncateSubmitTitle(strings.TrimSpace(title), 800),
	}}
	product.MultiLanguageDescList = []sheinproduct.LanguageContent{{
		Language: "en",
		Name:     truncateSubmitDescription(strings.TrimSpace(description), 5000),
	}}
}

func translateSubmitProductContent(product *sheinproduct.Product, api sheintranslateapi.TranslateAPI, region string) error {
	if product == nil {
		return nil
	}
	targetLanguages := submitprep.GetTargetLanguagesByRegion(strings.ToUpper(strings.TrimSpace(region)))
	if len(targetLanguages) == 0 {
		targetLanguages = []string{"en"}
	}

	var err error
	product.MultiLanguageNameList, err = submitprep.TranslateLocalizedList(product.MultiLanguageNameList, "", targetLanguages, api)
	if err != nil {
		return fmt.Errorf("translate SHEIN product name: %w", err)
	}
	product.MultiLanguageDescList, err = submitprep.TranslateLocalizedList(product.MultiLanguageDescList, "", targetLanguages, api)
	if err != nil {
		return fmt.Errorf("translate SHEIN product description: %w", err)
	}
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		fallback := strings.TrimSpace(skc.MultiLanguageName.Name)
		skc.MultiLanguageNameList, err = submitprep.TranslateLocalizedList(skc.MultiLanguageNameList, fallback, targetLanguages, api)
		if err != nil {
			return fmt.Errorf("translate SHEIN SKC name: %w", err)
		}
		if translated := submitprep.FindLanguageContent(skc.MultiLanguageNameList, "en"); translated != "" {
			skc.MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: translated}
		}
	}
	return nil
}

func truncateSubmitTitle(title string, maxLength int) string {
	if len(title) <= maxLength {
		return title
	}
	truncated := title[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 && lastSpace > maxLength-50 {
		truncated = truncated[:lastSpace]
	}
	return strings.TrimSpace(truncated)
}

func truncateSubmitDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}
	truncated := description[:maxLength]
	if lastPeriod := strings.LastIndexAny(truncated, ".!?"); lastPeriod > 0 && lastPeriod > maxLength-200 {
		truncated = truncated[:lastPeriod+1]
	}
	return strings.TrimSpace(truncated)
}

func toSubmitLocalizedTexts(items []sheinproduct.LanguageContent) []LocalizedText {
	if len(items) == 0 {
		return nil
	}
	out := make([]LocalizedText, 0, len(items))
	for _, item := range items {
		lang := strings.TrimSpace(item.Language)
		name := strings.TrimSpace(item.Name)
		if lang == "" || name == "" {
			continue
		}
		out = append(out, LocalizedText{Language: lang, Name: name})
	}
	return out
}

func countSubmitImages(product *sheinproduct.Product) int {
	if product == nil || product.ImageInfo == nil {
		return 0
	}
	return len(product.ImageInfo.ImageInfoList)
}
