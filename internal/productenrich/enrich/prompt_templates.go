package enrich

import (
	"encoding/json"
	"fmt"
	"strings"

	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/prompt"
)

func renderProductEnrichPrompt(key string, vars map[string]any, fallback string) string {
	if prompt.GlobalRegistry == nil {
		return fallback
	}

	rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback)
	if err != nil {
		return fallback
	}

	return rendered
}

func buildAnalysisSections(sections ...analysisSection) string {
	var sb strings.Builder
	for _, section := range sections {
		if !section.enabled {
			continue
		}
		sb.WriteString(section.title)
		sb.WriteString(":\n")
		sb.WriteString(section.content)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String())
}

type analysisSection struct {
	title   string
	content string
	enabled bool
}

func marshalPromptJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func buildProductJSONPrompt(analysis *productenrich.ProductAnalysis, fallback string) string {
	if analysis == nil {
		return fallback
	}

	sections := buildAnalysisSections(
		analysisSection{
			title:   "Product representation",
			content: marshalPromptJSON(analysis.Representation),
			enabled: analysis.Representation != nil,
		},
		analysisSection{
			title:   "Text attributes",
			content: marshalPromptJSON(analysis.TextAttributes),
			enabled: analysis.TextAttributes != nil,
		},
		analysisSection{
			title:   "Image attributes",
			content: marshalPromptJSON(analysis.ImageAttributes),
			enabled: analysis.ImageAttributes != nil,
		},
		analysisSection{
			title:   "1688 scraped data",
			content: marshalPromptJSON(analysis.ScrapedData),
			enabled: analysis.ScrapedData != nil,
		},
	)

	return renderProductEnrichPrompt(prompt.KProductEnrichGenerationProductJSON, map[string]any{
		"analysis_sections": sections,
	}, fallback)
}

func buildProductSpecsPrompt(analysis *productenrich.ProductAnalysis, fallback string) string {
	if analysis == nil {
		return fallback
	}

	sections := buildAnalysisSections(
		analysisSection{
			title:   "Representation",
			content: marshalPromptJSON(analysis.Representation),
			enabled: analysis.Representation != nil,
		},
		analysisSection{
			title:   "1688 scraped data",
			content: marshalPromptJSON(analysis.ScrapedData),
			enabled: analysis.ScrapedData != nil,
		},
	)

	return renderProductEnrichPrompt(prompt.KProductEnrichGenerationSpecs, map[string]any{
		"analysis_sections": sections,
	}, fallback)
}

func buildProductVariantsPrompt(analysis *productenrich.ProductAnalysis, fallback string) string {
	if analysis == nil {
		return fallback
	}

	sections := buildAnalysisSections(
		analysisSection{
			title:   "Representation",
			content: marshalPromptJSON(analysis.Representation),
			enabled: analysis.Representation != nil,
		},
		analysisSection{
			title:   "Image attributes",
			content: marshalPromptJSON(analysis.ImageAttributes),
			enabled: analysis.ImageAttributes != nil,
		},
		analysisSection{
			title:   "1688 scraped data",
			content: marshalPromptJSON(analysis.ScrapedData),
			enabled: analysis.ScrapedData != nil,
		},
	)

	return renderProductEnrichPrompt(prompt.KProductEnrichGenerationVariants, map[string]any{
		"analysis_sections": sections,
	}, fallback)
}

func buildExtractDimensionsPrompt(text string, fallback string) string {
	return renderProductEnrichPrompt(prompt.KProductEnrichGenerationExtractDimensions, map[string]any{
		"text": text,
	}, fallback)
}

func buildExtractWeightPrompt(text string, fallback string) string {
	return renderProductEnrichPrompt(prompt.KProductEnrichGenerationExtractWeight, map[string]any{
		"text": text,
	}, fallback)
}

func formatPromptSection(title string, value any) string {
	return fmt.Sprintf("%s:\n%s\n\n", title, marshalPromptJSON(value))
}
