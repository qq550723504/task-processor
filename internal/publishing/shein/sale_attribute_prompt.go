package shein

import "task-processor/internal/prompt"

var sheinSaleAttributePromptRenderer = prompt.NewTemplateRenderer()

func renderSheinSaleAttributePrompt(key string, fallback string, vars map[string]any) string {
	if prompt.GlobalRegistry != nil {
		if rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback); err == nil {
			return rendered
		}
	}
	rendered, err := sheinSaleAttributePromptRenderer.Render(fallback, vars)
	if err != nil {
		return fallback
	}
	return rendered
}
