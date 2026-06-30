package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

type sourceDimensionSelection struct {
	PrimarySourceDimension   string   `json:"primary_source_dimension"`
	SecondarySourceDimension string   `json:"secondary_source_dimension,omitempty"`
	Reasons                  []string `json:"reasons,omitempty"`
}

func selectSourceDimensions(dimensions []SourceVariantDimension, client TextGenerator) *sourceDimensionSelection {
	if len(dimensions) == 0 {
		return nil
	}
	if selection := selectSourceDimensionsWithLLM(dimensions, client); selection != nil {
		return selection
	}
	return selectSourceDimensionsFallback(dimensions)
}

func selectSourceDimensionsWithLLM(dimensions []SourceVariantDimension, client TextGenerator) *sourceDimensionSelection {
	if client == nil || len(dimensions) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := client.Generate(ctx, buildSourceDimensionSelectionPrompt(dimensions))
	if err != nil {
		return nil
	}
	response = jsonx.CleanLLMResponse(response)

	var selection sourceDimensionSelection
	if err := json.Unmarshal([]byte(response), &selection); err != nil {
		return nil
	}
	if strings.TrimSpace(selection.PrimarySourceDimension) == "" {
		return nil
	}
	if !sourceDimensionExists(dimensions, selection.PrimarySourceDimension) {
		return nil
	}
	if strings.TrimSpace(selection.SecondarySourceDimension) != "" &&
		!sourceDimensionExists(dimensions, selection.SecondarySourceDimension) {
		selection.SecondarySourceDimension = ""
	}
	return &selection
}

func buildSourceDimensionSelectionPrompt(dimensions []SourceVariantDimension) string {
	var dimensionBuilder strings.Builder
	for _, dimension := range dimensions {
		dimensionBuilder.WriteString(fmt.Sprintf("- name=%q distinct=%d values=%q\n", dimension.Name, dimension.DistinctCount, strings.Join(dimension.Values, " | ")))
	}
	return renderSheinSaleAttributePrompt(prompt.KSheinSaleAttributeSourceDimension, `You are choosing source sales dimensions for SHEIN draft grouping.
Choose exactly one preliminary primary_source_dimension and optionally one preliminary secondary_source_dimension from the source data.
This is only a source-data fallback before the live SHEIN template is known. Do not infer the final SHEIN primary sale attribute here.
Do not rename source dimensions. Prefer stable, structured, non-technical source dimensions over prompt text or identifiers.
Return JSON only with keys primary_source_dimension, secondary_source_dimension, reasons.

Source dimensions:
{{.SourceDimensionsBlock}}`, map[string]any{
		"SourceDimensionsBlock": dimensionBuilder.String(),
	})
}

func selectSourceDimensionsFallback(dimensions []SourceVariantDimension) *sourceDimensionSelection {
	selected := sheinmarketpub.SelectSourceDimensionsFallback(adaptSourceDimensionsForPolicy(dimensions))
	if selected == nil {
		return nil
	}
	selection := &sourceDimensionSelection{
		PrimarySourceDimension:   selected.PrimarySourceDimension,
		SecondarySourceDimension: selected.SecondarySourceDimension,
		Reasons:                  append([]string(nil), selected.Reasons...),
	}
	return selection
}

func sourceDimensionExists(dimensions []SourceVariantDimension, name string) bool {
	return sheinmarketpub.SourceDimensionExists(adaptSourceDimensionsForPolicy(dimensions), name)
}
