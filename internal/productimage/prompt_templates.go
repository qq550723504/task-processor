package productimage

import (
	"strings"

	"task-processor/internal/prompt"
)

type resolvedProductImagePrompt struct {
	Key     string
	Source  string
	Version string
	Text    string
}

func normalizeProductImagePromptKey(promptRef string, defaultKey string) string {
	normalized := strings.TrimSpace(promptRef)
	if normalized == "" {
		return defaultKey
	}
	replacer := strings.NewReplacer("/", ".", "-", "_")
	return replacer.Replace(normalized)
}

func normalizedFaithfulEditPromptRef(req *FaithfulEditRequest) string {
	if req == nil {
		return ""
	}
	switch req.Operation {
	case "extract_subject":
		return normalizeProductImagePromptKey(req.PromptRef, "productimage.subject.extract")
	case "render_white_background":
		return normalizeProductImagePromptKey(req.PromptRef, "productimage.white_background.default")
	default:
		return normalizeProductImagePromptKey(req.PromptRef, "")
	}
}

func normalizedScenePromptRef(req *SceneGenerationRequest) string {
	if req == nil {
		return ""
	}
	return normalizeProductImagePromptKey(req.PromptRef, "productimage.scene.default")
}

func resolveProductImagePrompt(promptRef string, defaultKey string, vars map[string]any, fallback string) resolvedProductImagePrompt {
	key := normalizeProductImagePromptKey(promptRef, defaultKey)
	resolved := resolvedProductImagePrompt{
		Key:     key,
		Source:  "fallback",
		Version: "default",
		Text:    fallback,
	}
	if prompt.GlobalRegistry == nil {
		return resolved
	}
	if !productImagePromptKeyExists(prompt.GlobalRegistry, key) {
		return resolved
	}

	rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback)
	if err != nil {
		return resolved
	}

	resolved.Source = "registry"
	resolved.Text = rendered
	return resolved
}

func renderProductImagePrompt(promptRef string, defaultKey string, vars map[string]any, fallback string) string {
	text := strings.TrimSpace(resolveProductImagePrompt(promptRef, defaultKey, vars, fallback).Text)
	if text == "" {
		return fallback
	}
	return text
}

func productImagePromptKeyExists(registry prompt.PromptRegistry, key string) bool {
	if registry == nil {
		return false
	}
	for _, candidate := range registry.Keys() {
		if candidate == key {
			return true
		}
	}
	return false
}
