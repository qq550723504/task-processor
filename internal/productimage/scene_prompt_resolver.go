package productimage

import (
	"strings"

	"task-processor/internal/prompt"
)

type scenePromptOptions struct {
	Marketplace     string
	Category        string
	SceneStyle      string
	BackgroundTone  string
	Composition     string
	PropsLevel      string
	AudienceHint    string
	CustomSceneHint string
	DefaultsSource  string
}

func buildSceneGenerationRequest(asset *ImageAsset, context *ProductContext) *SceneGenerationRequest {
	options := resolveScenePromptOptions(nil, context)
	return &SceneGenerationRequest{
		SourceAsset:     asset,
		ProductContext:  context,
		PromptRef:       resolveScenePromptKey("", options.Category),
		SceneIntent:     "gallery_scene",
		SceneCategory:   options.Category,
		SceneStyle:      options.SceneStyle,
		BackgroundTone:  options.BackgroundTone,
		Composition:     options.Composition,
		PropsLevel:      options.PropsLevel,
		AudienceHint:    options.AudienceHint,
		CustomSceneHint: options.CustomSceneHint,
	}
}

func resolveScenePromptOptions(req *SceneGenerationRequest, context *ProductContext) scenePromptOptions {
	marketplace := firstNonEmpty(
		contextAttribute(context, "marketplace"),
		contextAttribute(context, "source_marketplace"),
	)
	category := resolveSceneCategory(req, context)
	preset := resolveScenePreset(marketplace, category)
	explicit := &SceneGenerationOptions{
		SceneCategory:   firstNonEmpty(trimmed(reqField(req, "scene_category")), contextAttribute(context, "scene_category")),
		SceneStyle:      firstNonEmpty(trimmed(reqField(req, "scene_style")), contextAttribute(context, "scene_style")),
		BackgroundTone:  firstNonEmpty(trimmed(reqField(req, "background_tone")), contextAttribute(context, "background_tone")),
		Composition:     firstNonEmpty(trimmed(reqField(req, "composition")), contextAttribute(context, "composition")),
		PropsLevel:      firstNonEmpty(trimmed(reqField(req, "props_level")), contextAttribute(context, "props_level")),
		AudienceHint:    firstNonEmpty(trimmed(reqField(req, "audience_hint")), contextAttribute(context, "audience_hint")),
		CustomSceneHint: firstNonEmpty(trimmed(reqField(req, "custom_scene_hint")), contextAttribute(context, "custom_scene_hint")),
	}
	merged := MergeSceneGenerationOptions(preset.Options, explicit)
	if merged == nil {
		merged = &SceneGenerationOptions{SceneCategory: category}
	}
	defaultsSource := preset.Source
	if defaultsSource == "" {
		defaultsSource = "fallback"
	}
	if explicit != nil && !explicit.IsEmpty() {
		defaultsSource = "explicit"
	}
	return scenePromptOptions{
		Marketplace:     marketplace,
		Category:        firstNonEmpty(merged.SceneCategory, category),
		SceneStyle:      merged.SceneStyle,
		BackgroundTone:  merged.BackgroundTone,
		Composition:     merged.Composition,
		PropsLevel:      merged.PropsLevel,
		AudienceHint:    merged.AudienceHint,
		CustomSceneHint: merged.CustomSceneHint,
		DefaultsSource:  defaultsSource,
	}
}

func resolveScenePromptKey(promptRef string, category string) string {
	if normalized := strings.TrimSpace(promptRef); normalized != "" {
		return normalizeProductImagePromptKey(normalized, prompt.KProductImageSceneDefault)
	}
	if category != "" {
		return scenePromptKeyForCategory(category)
	}
	return prompt.KProductImageSceneDefault
}

func resolveScenePromptCandidateKeys(req *SceneGenerationRequest, options scenePromptOptions) []string {
	if req != nil && strings.TrimSpace(req.PromptRef) != "" {
		return []string{normalizeProductImagePromptKey(req.PromptRef, prompt.KProductImageSceneDefault)}
	}
	keys := make([]string, 0, 2)
	if options.Category != "" {
		keys = append(keys, scenePromptKeyForCategory(options.Category))
	}
	keys = append(keys, prompt.KProductImageSceneDefault)
	return keys
}

func scenePromptKeyForCategory(category string) string {
	switch category {
	case "shoes":
		return prompt.KProductImageSceneShoes
	case "jewelry":
		return prompt.KProductImageSceneJewelry
	case "bags":
		return prompt.KProductImageSceneBags
	default:
		return "productimage.scene." + category
	}
}

func resolveSceneCategory(req *SceneGenerationRequest, context *ProductContext) string {
	candidates := []string{}
	if req != nil {
		candidates = append(candidates, req.SceneCategory)
	}
	candidates = append(candidates,
		contextAttribute(context, "scene_category"),
		contextAttribute(context, "category"),
		contextAttribute(context, "product_category"),
	)
	if context != nil {
		candidates = append(candidates, context.ProductType, context.Title, context.ScrapedTitle)
	}
	for _, candidate := range candidates {
		if normalized := normalizeSceneCategory(candidate); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeSceneCategory(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	switch {
	case containsAny(normalized, "sneaker", "shoe", "shoes", "boot", "sandal", "slipper", "heel", "loafer"):
		return "shoes"
	case containsAny(normalized, "necklace", "ring", "earring", "earrings", "bracelet", "jewelry", "jewellery", "pendant", "brooch"):
		return "jewelry"
	case containsAny(normalized, "handbag", "backpack", "bag", "bags", "purse", "tote", "satchel", "crossbody"):
		return "bags"
	default:
		return ""
	}
}

func contextAttribute(context *ProductContext, key string) string {
	if context == nil || context.Attributes == nil {
		return ""
	}
	return trimmed(context.Attributes[key])
}

func reqField(req *SceneGenerationRequest, key string) string {
	if req == nil {
		return ""
	}
	switch key {
	case "scene_style":
		return req.SceneStyle
	case "background_tone":
		return req.BackgroundTone
	case "composition":
		return req.Composition
	case "props_level":
		return req.PropsLevel
	case "audience_hint":
		return req.AudienceHint
	case "custom_scene_hint":
		return req.CustomSceneHint
	case "scene_category":
		return req.SceneCategory
	default:
		return ""
	}
}

func trimmed(value string) string {
	return strings.TrimSpace(value)
}
