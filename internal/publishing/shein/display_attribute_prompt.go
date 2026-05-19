package shein

import (
	"context"

	"task-processor/internal/prompt"
)

var sheinDisplayAttributePromptRenderer = prompt.NewTemplateRenderer()

func renderSheinDisplayAttributePrompt(key string, fallback string, vars map[string]any) string {
	if prompt.GlobalRegistry != nil {
		if rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback); err == nil {
			return rendered
		}
	}
	rendered, err := sheinDisplayAttributePromptRenderer.Render(fallback, vars)
	if err != nil {
		return fallback
	}
	return rendered
}

func contextWithFallback(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
