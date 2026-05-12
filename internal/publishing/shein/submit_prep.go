package shein

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
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
	_ = ctx
	_ = aiClient
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
	// Submit-time payloads need to stay deterministic. The workbench preview/final
	// draft is already the reviewed content surface; re-running a multimodal
	// optimizer here makes publish and save_draft diverge on the same task.
	applySubmitContent(product, title, description)
	return nil
}

func optimizeSubmitContentWithAI(ctx context.Context, aiClient openaiclient.ChatCompleter, title, description, features string, imageURLs []string) (string, string, error) {
	systemPrompt := `You are an e-commerce content optimization expert for SHEIN product listings.
Requirements:
1. Output title and description in natural English only.
2. Optimize for e-commerce conversion, search relevance, and attribute clarity instead of being minimal.
3. Use the provided product images to identify visual motifs, style, room or use context, print or pattern cues, and other concrete shopper-relevant details that are visible.
4. Title should usually be 110-220 characters when the source supports it, with high-intent product keywords, material or construction terms, style cues, use-case context, and shopper-friendly wording.
5. Description should usually be 220-900 characters when the source supports it, with 3-5 compact sentences covering what the product is, key material or build details, visual style, common use scenarios, and why a shopper would choose it.
6. Keep claims concrete and product-focused. Avoid fluff, repetition, keyword stuffing, brand names, emojis, medical claims, absolute guarantees, or platform policy-risk claims.
7. Preserve the core product type and major factual details from the source.
8. Return strict JSON only: {"title":"...","description":"..."}`
	userPrompt := fmt.Sprintf("Source product content:\nTitle: %s\nDescription: %s\nFeatures:\n%s\n\nCreate a stronger SHEIN listing title and description aimed at ecommerce conversion. Use the images as additional evidence for visible style, pattern, room context, and shopper intent. Do not invent hidden materials, dimensions, or compliance claims.", title, description, features)
	temperature := float32(0.7)
	messages := []openaiclient.ChatCompletionMessage{{
		Role:    "system",
		Content: systemPrompt,
	}}
	if len(imageURLs) == 0 {
		messages = append(messages, openaiclient.ChatCompletionMessage{
			Role:    "user",
			Content: userPrompt,
		})
	} else {
		parts := make([]openaiclient.ChatCompletionContentPart, 0, 1+len(imageURLs))
		parts = append(parts, openaiclient.ChatCompletionContentPart{
			Type: "text",
			Text: userPrompt,
		})
		for _, imageURL := range imageURLs {
			parts = append(parts, openaiclient.ChatCompletionContentPart{
				Type: "image_url",
				ImageURL: &openaiclient.ChatCompletionContentPartImage{
					URL:    imageURL,
					Detail: "auto",
				},
			})
		}
		messages = append(messages, openaiclient.ChatCompletionMessage{
			Role:         "user",
			MultiContent: parts,
		})
	}
	resp, err := aiClient.CreateChatCompletion(ctx, &openaiclient.ChatCompletionRequest{
		Model:       aiClient.GetDefaultModel(),
		Temperature: &temperature,
		Messages:    messages,
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

func collectSubmitContentImageURLs(product *sheinproduct.Product) []string {
	if product == nil {
		return nil
	}
	if product.ImageInfo != nil {
		for _, image := range product.ImageInfo.ImageInfoList {
			if imageURL := strings.TrimSpace(image.ImageURL); imageURL != "" {
				return []string{imageURL}
			}
		}
	}
	for _, skc := range product.SKCList {
		for _, image := range skc.ImageInfo.ImageInfoList {
			if imageURL := strings.TrimSpace(image.ImageURL); imageURL != "" {
				return []string{imageURL}
			}
		}
	}
	return nil
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
	title = truncateSubmitTitle(strings.TrimSpace(title), 800)
	description = truncateSubmitDescription(strings.TrimSpace(description), 5000)
	product.MultiLanguageNameList = []sheinproduct.LanguageContent{{
		Language: "en",
		Name:     title,
	}}
	product.MultiLanguageDescList = []sheinproduct.LanguageContent{{
		Language: "en",
		Name:     description,
	}}
	applySubmitSKCContent(product, title)
}

func applySubmitSKCContent(product *sheinproduct.Product, title string) {
	if product == nil {
		return
	}
	for skcIndex := range product.SKCList {
		skc := &product.SKCList[skcIndex]
		suffix := submitprep.FirstLocalizedText(skc.MultiLanguageNameList)
		if strings.TrimSpace(suffix) == "" {
			suffix = strings.TrimSpace(skc.MultiLanguageName.Name)
		}
		name := buildSubmitSKCTitle(title, suffix)
		skc.MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: name}
		skc.MultiLanguageNameList = []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     name,
		}}
	}
}

func buildSubmitSKCTitle(title, suffix string) string {
	title = strings.TrimSpace(title)
	suffix = strings.TrimSpace(suffix)
	if title == "" {
		return truncateSubmitTitle(suffix, 200)
	}
	if suffix == "" {
		return truncateSubmitTitle(title, 200)
	}
	if strings.Contains(strings.ToLower(title), strings.ToLower(suffix)) {
		return truncateSubmitTitle(title, 200)
	}
	return truncateSubmitTitle(title+" - "+suffix, 200)
}

func strengthenSubmitTitle(title, sourceTitle, sourceDescription string) string {
	title = strings.TrimSpace(title)
	if len(title) >= 90 {
		return truncateSubmitTitle(title, 800)
	}
	extra := firstSubmitSentence(sourceDescription)
	if extra == "" {
		extra = strings.TrimSpace(sourceTitle)
	}
	extra = strings.TrimSpace(extra)
	if submitprep.TextNeedsTranslation(extra, "en") {
		extra = ""
	}
	if extra == "" || strings.Contains(strings.ToLower(title), strings.ToLower(extra)) {
		return truncateSubmitTitle(title, 800)
	}
	return truncateSubmitTitle(title+" - "+extra, 800)
}

func strengthenSubmitDescription(description, sourceDescription string) string {
	description = strings.TrimSpace(description)
	if len(description) >= 220 {
		return truncateSubmitDescription(description, 5000)
	}
	extra := strings.TrimSpace(sourceDescription)
	if submitprep.TextNeedsTranslation(extra, "en") {
		extra = ""
	}
	if extra == "" || strings.Contains(strings.ToLower(description), strings.ToLower(extra)) {
		return truncateSubmitDescription(description, 5000)
	}
	joined := description
	if joined != "" && !strings.HasSuffix(joined, ".") {
		joined += "."
	}
	if joined != "" {
		joined += " "
	}
	joined += extra
	return truncateSubmitDescription(strings.TrimSpace(joined), 5000)
}

func firstSubmitSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if index := strings.IndexAny(text, ".!?;\n"); index > 0 {
		text = text[:index]
	}
	return strings.TrimSpace(text)
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
