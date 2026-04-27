package listingkit

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/content"
)

func optimizeSheinProductContentForSubmit(ctx context.Context, product *sheinproduct.Product, aiClient openaiclient.ChatCompleter) error {
	if product == nil {
		return nil
	}
	title := firstSheinLocalizedText(product.MultiLanguageNameList)
	description := firstSheinLocalizedText(product.MultiLanguageDescList)
	if strings.TrimSpace(title) == "" && strings.TrimSpace(description) == "" {
		return nil
	}
	if aiClient == nil {
		if sheinTextNeedsTranslation(title, "en") || sheinTextNeedsTranslation(description, "en") {
			return fmt.Errorf("shein content optimizer is not configured")
		}
		return nil
	}

	cleaner := content.NewTextCleaner()
	title = cleaner.NormalizeText(title)
	description = cleaner.NormalizeText(description)
	if title == "" {
		title = "Quality Home Decor Product"
	}
	if description == "" {
		description = title
	}

	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	optimizedTitle, optimizedDescription, err := optimizeSheinSubmitContentWithAI(aiCtx, aiClient, title, description, buildSheinSubmitContentFeatures(product))
	if err != nil {
		return err
	}
	optimizedTitle = strings.TrimSpace(cleaner.RemoveForbiddenWords(cleaner.NormalizeText(optimizedTitle)))
	optimizedDescription = strings.TrimSpace(cleaner.RemoveForbiddenWords(cleaner.NormalizeText(optimizedDescription)))
	if optimizedTitle == "" {
		optimizedTitle = title
	}
	if optimizedDescription == "" {
		optimizedDescription = description
	}
	optimizedTitle = truncateSheinSubmitTitle(optimizedTitle, 800)
	optimizedDescription = truncateSheinSubmitDescription(optimizedDescription, 5000)

	product.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: optimizedTitle}}
	product.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: optimizedDescription}}
	return nil
}

func optimizeSheinSubmitContentWithAI(ctx context.Context, aiClient openaiclient.ChatCompleter, title, description, features string) (string, string, error) {
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

func buildSheinSubmitContentFeatures(product *sheinproduct.Product) string {
	if product == nil {
		return ""
	}
	parts := make([]string, 0, 8)
	if product.CategoryID > 0 {
		parts = append(parts, fmt.Sprintf("SHEIN category id: %d", product.CategoryID))
	}
	for _, skc := range product.SKCList {
		if text := firstSheinLocalizedText(skc.MultiLanguageNameList); text != "" {
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

func truncateSheinSubmitTitle(title string, maxLength int) string {
	if len(title) <= maxLength {
		return title
	}
	truncated := title[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 && lastSpace > maxLength-50 {
		truncated = truncated[:lastSpace]
	}
	return strings.TrimSpace(truncated)
}

func truncateSheinSubmitDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}
	truncated := description[:maxLength]
	if lastPeriod := strings.LastIndexAny(truncated, ".!?"); lastPeriod > 0 && lastPeriod > maxLength-200 {
		truncated = truncated[:lastPeriod+1]
	}
	return strings.TrimSpace(truncated)
}
