package productimage

import (
	"strings"

	"task-processor/internal/prompt"
)

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

func renderProductImagePrompt(promptRef string, defaultKey string, vars map[string]any, fallback string) string {
	if prompt.GlobalRegistry == nil {
		return fallback
	}
	key := normalizeProductImagePromptKey(promptRef, defaultKey)
	rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback)
	if err != nil {
		return fallback
	}
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return fallback
	}
	return rendered
}
