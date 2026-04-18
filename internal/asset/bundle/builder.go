package bundle

import (
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	common "task-processor/internal/publishing/common"
)

type BuildRequest struct {
	Platform  string
	Inventory *asset.Inventory
	Recipes   []assetrecipe.AssetRecipe
}

type Builder interface {
	Build(req BuildRequest) *common.PublishImageBundle
}

type DefaultBuilder struct{}

func NewBuilder() *DefaultBuilder { return &DefaultBuilder{} }

func (b *DefaultBuilder) Build(req BuildRequest) *common.PublishImageBundle {
	out := &common.PublishImageBundle{
		Platform: strings.TrimSpace(req.Platform),
	}
	if req.Inventory == nil {
		out.Warnings = append(out.Warnings, "asset inventory is empty")
		return out
	}
	selected := map[string]struct{}{}
	for _, recipe := range req.Recipes {
		if recipe.Template == nil {
			continue
		}
		out.RecipeIDs = append(out.RecipeIDs, recipe.ID)
		candidates := findCandidates(req.Inventory.Records, recipe.Template.PreferredKinds, selected, recipe.Template.MaxItems)
		switch recipe.Template.BundleSlot {
		case "main":
			if len(candidates) == 0 {
				out.MissingSlots = append(out.MissingSlots, missingSlot(recipe, "no matching asset found"))
				if recipe.Generated {
					out.PendingGeneration = append(out.PendingGeneration, plannedGenerationTask(req.Platform, req.Inventory, recipe))
				}
				if !recipe.Template.Optional {
					out.Warnings = append(out.Warnings, "missing required main image")
				}
				continue
			}
			slot := buildSlot(recipe, candidates[0])
			out.Main = &slot
			selected[candidates[0].ID] = struct{}{}
			out.SelectedAssetIDs = append(out.SelectedAssetIDs, candidates[0].ID)
			if recipe.Generated && candidateIsFallback(recipe, candidates[0]) {
				out.MissingSlots = append(out.MissingSlots, missingSlot(recipe, "selected fallback asset"))
				out.PendingGeneration = append(out.PendingGeneration, plannedGenerationTask(req.Platform, req.Inventory, recipe))
			}
		case "gallery":
			if len(candidates) == 0 && recipe.Generated {
				out.PendingGeneration = append(out.PendingGeneration, plannedGenerationTask(req.Platform, req.Inventory, recipe))
			}
			for _, candidate := range candidates {
				slot := buildSlot(recipe, candidate)
				out.Gallery = append(out.Gallery, slot)
				selected[candidate.ID] = struct{}{}
				out.SelectedAssetIDs = append(out.SelectedAssetIDs, candidate.ID)
			}
		default:
			if len(candidates) == 0 && recipe.Generated {
				out.PendingGeneration = append(out.PendingGeneration, plannedGenerationTask(req.Platform, req.Inventory, recipe))
			}
			for _, candidate := range candidates {
				slot := buildSlot(recipe, candidate)
				out.Auxiliary = append(out.Auxiliary, slot)
				selected[candidate.ID] = struct{}{}
				out.SelectedAssetIDs = append(out.SelectedAssetIDs, candidate.ID)
			}
		}
	}
	out.RecipeIDs = uniqueStrings(out.RecipeIDs)
	out.SelectedAssetIDs = uniqueStrings(out.SelectedAssetIDs)
	return out
}

func missingSlot(recipe assetrecipe.AssetRecipe, reason string) common.MissingSlot {
	slot := ""
	purpose := ""
	templateLabel := ""
	renderProfile := ""
	optional := false
	if recipe.Template != nil {
		slot = recipe.Template.BundleSlot
		purpose = recipe.Template.Purpose
		templateLabel = firstNonEmpty(recipe.Template.TemplateLabel, recipe.Name, recipe.ID)
		renderProfile = recipe.Template.RenderProfile
		optional = recipe.Template.Optional
	}
	return common.MissingSlot{
		Slot:          slot,
		Purpose:       purpose,
		RecipeID:      recipe.ID,
		TemplateLabel: templateLabel,
		RenderProfile: renderProfile,
		StateLabel:    "missing",
		Reason:        reason,
		Optional:      optional,
	}
}

func plannedGenerationTask(platform string, inventory *asset.Inventory, recipe assetrecipe.AssetRecipe) assetgeneration.Task {
	return assetgeneration.Task{
		ID:              generationTaskID(platform, recipe.ID),
		Platform:        platform,
		RecipeID:        recipe.ID,
		AssetKind:       recipe.AssetKind,
		Slot:            recipeSlot(recipe),
		Purpose:         recipePurpose(recipe),
		TemplateLabel:   recipeTemplateLabel(recipe),
		RenderProfile:   recipeRenderProfile(recipe),
		Status:          "planned",
		ExecutionStatus: "planned",
		ExecutionMode:   assetgeneration.PlannedExecutionMode(recipe.AssetKind),
		CanExecute:      recipe.Generated,
		Lineage:         []string{platform, recipe.ID},
		SourceAssetIDs:  candidateSourceAssetIDs(inventory),
	}
}

func generationTaskID(platform, recipeID string) string {
	return strings.TrimSpace(platform) + ":" + strings.TrimSpace(recipeID)
}

func recipeSlot(recipe assetrecipe.AssetRecipe) string {
	if recipe.Template == nil {
		return ""
	}
	return recipe.Template.BundleSlot
}

func recipePurpose(recipe assetrecipe.AssetRecipe) string {
	if recipe.Template == nil {
		return ""
	}
	return recipe.Template.Purpose
}

func recipeTemplateLabel(recipe assetrecipe.AssetRecipe) string {
	if recipe.Template == nil {
		return ""
	}
	return firstNonEmpty(recipe.Template.TemplateLabel, recipe.Name, recipe.ID)
}

func recipeRenderProfile(recipe assetrecipe.AssetRecipe) string {
	if recipe.Template == nil {
		return strings.ReplaceAll(strings.TrimSpace(recipe.ID), "-", "_")
	}
	return firstNonEmpty(recipe.Template.RenderProfile, strings.ReplaceAll(strings.TrimSpace(recipe.ID), "-", "_"))
}

func candidateSourceAssetIDs(inventory *asset.Inventory) []string {
	if inventory == nil {
		return nil
	}
	out := make([]string, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		if record.Kind == asset.KindSourceImage || record.Kind == asset.KindMainImage || record.Kind == asset.KindCleanImage || record.Kind == asset.KindSubjectCutout {
			out = append(out, record.ID)
		}
	}
	return uniqueStrings(out)
}

func candidateIsFallback(recipe assetrecipe.AssetRecipe, record asset.AssetRecord) bool {
	if recipe.AssetKind == "" {
		return false
	}
	return record.Kind != recipe.AssetKind
}

func taskExecutionMode(recipe assetrecipe.AssetRecipe) string {
	return assetgeneration.PlannedExecutionMode(recipe.AssetKind)
}

func buildSlot(recipe assetrecipe.AssetRecipe, record asset.AssetRecord) common.BundleSlot {
	satisfiedBy := "exact_asset"
	fallbackFrom := ""
	executionStatus := "ready"
	stateLabel := "ready"
	retryHint := ""
	if candidateIsFallback(recipe, record) {
		satisfiedBy = "fallback_asset"
		fallbackFrom = string(recipe.AssetKind)
		executionStatus = "fallback"
		stateLabel = "fallback_in_use"
		retryHint = "retry generation for this slot to replace the fallback asset"
	}
	return common.BundleSlot{
		Key:             firstNonEmpty(recipe.Template.BundleSlot, recipe.Template.Purpose),
		Purpose:         recipe.Template.Purpose,
		IdealKind:       string(recipe.AssetKind),
		TemplateLabel:   firstNonEmpty(recipe.Template.TemplateLabel, recipe.Name, recipe.ID),
		StateLabel:      stateLabel,
		RetryHint:       retryHint,
		AssetID:         record.ID,
		URL:             record.URL,
		Kind:            string(record.Kind),
		RecipeID:        recipe.ID,
		SatisfiedBy:     satisfiedBy,
		FallbackFrom:    fallbackFrom,
		ExecutionStatus: executionStatus,
		SourceAssetIDs:  append([]string(nil), sourceAssetIDs(record)...),
	}
}

func sourceAssetIDs(record asset.AssetRecord) []string {
	if record.Lineage == nil {
		return nil
	}
	return append([]string(nil), record.Lineage.SourceAssetIDs...)
}

func findCandidates(records []asset.AssetRecord, preferred []asset.Kind, selected map[string]struct{}, maxItems int) []asset.AssetRecord {
	if maxItems <= 0 {
		maxItems = 1
	}
	for _, kind := range preferred {
		out := make([]asset.AssetRecord, 0, maxItems)
		for _, record := range records {
			if record.Kind != kind {
				continue
			}
			if _, ok := selected[record.ID]; ok {
				continue
			}
			out = append(out, record)
			if len(out) >= maxItems {
				return out
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
