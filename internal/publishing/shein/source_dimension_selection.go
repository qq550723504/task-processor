package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/prompt"
)

type sourceDimensionSelection struct {
	PrimarySourceDimension   string   `json:"primary_source_dimension"`
	SecondarySourceDimension string   `json:"secondary_source_dimension,omitempty"`
	Reasons                  []string `json:"reasons,omitempty"`
}

func selectSourceDimensions(dimensions []SourceVariantDimension, client openaiclient.ChatCompleter) *sourceDimensionSelection {
	if len(dimensions) == 0 {
		return nil
	}
	if selection := selectSourceDimensionsWithLLM(dimensions, client); selection != nil {
		return selection
	}
	return selectSourceDimensionsFallback(dimensions)
}

func selectSourceDimensionsWithLLM(dimensions []SourceVariantDimension, client openaiclient.ChatCompleter) *sourceDimensionSelection {
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
Choose exactly one primary_source_dimension for SKC grouping and optionally one secondary_source_dimension for SKU grouping.
Do not rename source dimensions. Prefer the most product-defining dimension as primary.
Return JSON only with keys primary_source_dimension, secondary_source_dimension, reasons.

Source dimensions:
{{.SourceDimensionsBlock}}`, map[string]any{
		"SourceDimensionsBlock": dimensionBuilder.String(),
	})
}

func selectSourceDimensionsFallback(dimensions []SourceVariantDimension) *sourceDimensionSelection {
	ranked := append([]SourceVariantDimension(nil), dimensions...)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if sourceDimensionPrimaryPriority(a) != sourceDimensionPrimaryPriority(b) {
			return sourceDimensionPrimaryPriority(a) > sourceDimensionPrimaryPriority(b)
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return normalizeText(a.Name) < normalizeText(b.Name)
	})
	selection := &sourceDimensionSelection{
		PrimarySourceDimension: ranked[0].Name,
		Reasons:                []string{"SHEIN 模板未就绪，先按源销售属性维度生成最小分组计划"},
	}
	secondaryPool := append([]SourceVariantDimension(nil), ranked[1:]...)
	sort.SliceStable(secondaryPool, func(i, j int) bool {
		a, b := secondaryPool[i], secondaryPool[j]
		if sourceDimensionSecondaryPriority(a) != sourceDimensionSecondaryPriority(b) {
			return sourceDimensionSecondaryPriority(a) > sourceDimensionSecondaryPriority(b)
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return normalizeText(a.Name) < normalizeText(b.Name)
	})
	for _, dimension := range secondaryPool {
		if dimension.Name == selection.PrimarySourceDimension {
			continue
		}
		selection.SecondarySourceDimension = dimension.Name
		break
	}
	return selection
}

func sourceDimensionExists(dimensions []SourceVariantDimension, name string) bool {
	name = normalizeText(name)
	for _, dimension := range dimensions {
		if normalizeText(dimension.Name) == name {
			return true
		}
	}
	return false
}
