package referenceanalysis

import (
	"encoding/json"
	"strings"
)

func Interpret(rawAnalyses []string) (Result, error) {
	parsed := make([]imageAnalysis, 0, len(rawAnalyses))
	for _, raw := range rawAnalyses {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parsed = append(parsed, parseImageAnalysis(raw))
	}
	if len(parsed) == 0 {
		return Result{}, ErrNoInput
	}
	abstracted := abstractStudioReferenceAnalyses(parsed)
	if !hasStudioReusableSafeStyleDirection(abstracted) {
		return Result{}, ErrNoSafeDirection
	}
	result := Result{
		StyleBrief:        buildStudioReferenceStyleBrief(abstracted),
		SanitizedPrompt:   buildSanitizedStudioReferencePrompt(abstracted),
		HadUnsafeInput:    studioReferenceContainsUnsafeSignals(parsed, abstracted),
		HadMalformedInput: studioReferenceContainsMalformedFallback(abstracted),
	}
	if strings.TrimSpace(result.SanitizedPrompt) == "" {
		return Result{}, ErrEmptyPrompt
	}
	return result, nil
}

func parseImageAnalysis(raw string) imageAnalysis {
	cleaned := strings.TrimSpace(raw)
	var analysis imageAnalysis
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return imageAnalysis{Raw: cleaned}
	}
	analysis.Raw = cleaned
	return analysis
}

func buildStudioReferenceStyleBrief(analyses []abstractedAnalysis) string {
	parts := []string{"Reference style cues."}
	for _, item := range analyses {
		if item.Motif != "" {
			parts = append(parts, "Motif family: "+item.Motif+".")
		}
		if len(item.Palette) > 0 {
			parts = append(parts, "Palette direction: "+strings.Join(item.Palette, ", ")+".")
		}
		if len(item.Composition) > 0 {
			parts = append(parts, "Composition family: "+strings.Join(item.Composition, ", ")+".")
		}
		if item.Typography != "" {
			parts = append(parts, "Typography feel: "+item.Typography+".")
		}
		if item.Density != "" {
			parts = append(parts, "Visual density: "+item.Density+".")
		}
		if item.ProductFit != "" {
			parts = append(parts, "Product fit: "+item.ProductFit+".")
		}
		if item.Mood != "" {
			parts = append(parts, "Mood cue: "+item.Mood+".")
		}
		if item.GarmentPlacement != "" {
			parts = append(parts, "Garment placement: "+item.GarmentPlacement+".")
		}
	}
	return strings.Join(parts, " ")
}

func buildSanitizedStudioReferencePrompt(analyses []abstractedAnalysis) string {
	parts := []string{"Create an original POD artwork with a commercially proven graphic style direction."}
	if motifs := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.Motif
	}); len(motifs) > 0 {
		parts = append(parts, "Motif direction: "+strings.Join(motifs, ", ")+".")
	}
	if palettes := collectStudioReferencePalettes(analyses); len(palettes) > 0 {
		parts = append(parts, "Palette direction: "+strings.Join(palettes, ", ")+".")
	}
	if compositions := collectStudioReferenceCompositionFragments(analyses); len(compositions) > 0 {
		parts = append(parts, "Composition direction: "+strings.Join(compositions, ", ")+".")
	}
	if typography := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.Typography
	}); len(typography) > 0 {
		parts = append(parts, "Typography feel: "+strings.Join(typography, ", ")+".")
	}
	if density := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.Density
	}); len(density) > 0 {
		parts = append(parts, "Visual density: "+strings.Join(density, ", ")+".")
	}
	if productFit := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.ProductFit
	}); len(productFit) > 0 {
		parts = append(parts, "Product fit: "+strings.Join(productFit, ", ")+".")
	}
	if mood := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.Mood
	}); len(mood) > 0 {
		parts = append(parts, "Mood cue: "+strings.Join(mood, ", ")+".")
	}
	if garmentPlacement := collectStudioReferenceFragments(analyses, func(item abstractedAnalysis) string {
		return item.GarmentPlacement
	}); len(garmentPlacement) > 0 {
		parts = append(parts, "Garment placement: "+strings.Join(garmentPlacement, ", ")+".")
	}
	parts = append(parts, "Keep all graphics brand-neutral, use fresh custom wording if text appears, avoid recognizable characters or people, and use a clearly original layout.")
	return strings.Join(parts, " ")
}

func collectStudioReferenceFragments(analyses []abstractedAnalysis, pick func(abstractedAnalysis) string) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		value := strings.TrimSpace(pick(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func collectStudioReferencePalettes(analyses []abstractedAnalysis) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		if len(item.Palette) == 0 {
			continue
		}
		palette := strings.Join(item.Palette, ", ")
		if _, ok := seen[palette]; ok {
			continue
		}
		seen[palette] = struct{}{}
		result = append(result, palette)
	}
	return result
}

func collectStudioReferenceCompositionFragments(analyses []abstractedAnalysis) []string {
	result := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range analyses {
		for _, composition := range item.Composition {
			composition = strings.TrimSpace(composition)
			if composition == "" {
				continue
			}
			if _, ok := seen[composition]; ok {
				continue
			}
			seen[composition] = struct{}{}
			result = append(result, composition)
		}
	}
	return result
}
