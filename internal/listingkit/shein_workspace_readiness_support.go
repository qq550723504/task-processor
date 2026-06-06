package listingkit

import (
	listingworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

type SheinReadinessReason struct {
	Code     string `json:"code,omitempty"`
	Category string `json:"category,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type SheinRepairHint struct {
	Action        string                        `json:"action,omitempty"`
	Priority      string                        `json:"priority,omitempty"`
	Target        string                        `json:"target,omitempty"`
	EditorSection string                        `json:"editor_section,omitempty"`
	EditorFocus   []string                      `json:"editor_focus,omitempty"`
	RevisionPath  string                        `json:"revision_path,omitempty"`
	Description   string                        `json:"description,omitempty"`
	FieldPaths    []string                      `json:"field_paths,omitempty"`
	Patch         *SheinRepairPatchPayload      `json:"patch,omitempty"`
	Skeleton      *SheinEditorRevisionSkeleton  `json:"skeleton,omitempty"`
	Revision      *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation    *SheinRepairValidationPreview `json:"validation,omitempty"`
}

type sheinReadinessGuidance struct {
	reason      *SheinReadinessReason
	repairHints []SheinRepairHint
}

func buildSheinReadinessReason(spec *listingworkspace.ReadinessReasonSpec) *SheinReadinessReason {
	if spec == nil {
		return nil
	}
	return &SheinReadinessReason{
		Code:     spec.Code,
		Category: spec.Category,
		Summary:  spec.Summary,
	}
}

func buildSheinReadinessPatchPayload(pkg *SheinPackage, key string) *SheinRepairPatchPayload {
	switch key {
	case "category", "category_review":
		return &SheinRepairPatchPayload{
			CategoryResolution: buildSheinCategoryResolutionPatch(pkg),
		}
	case "attributes", "attribute_review":
		return &SheinRepairPatchPayload{
			AttributeResolution: buildSheinAttributeResolutionPatch(pkg),
		}
	case "sale_attributes", "variants":
		return &SheinRepairPatchPayload{
			SaleAttributeResolution: buildSheinSaleAttributeResolutionPatch(pkg),
			SKCPatches:              buildSheinEditorSKCPatches(pkg),
		}
	case "images":
		return &SheinRepairPatchPayload{
			Images: clonePlatformImageSetForEditor(pkg.Images),
		}
	case "manual_notes":
		return &SheinRepairPatchPayload{
			ReviewNotes: append([]string(nil), pkg.ReviewNotes...),
		}
	default:
		return nil
	}
}

func buildSheinReadinessRepairHint(pkg *SheinPackage, action string, fieldPaths []string, hint listingworkspace.ReadinessHintSpec, patch *SheinRepairPatchPayload) SheinRepairHint {
	artifacts := buildSheinRepairArtifacts(pkg, action, hint.EditorSection, patch)
	return SheinRepairHint{
		Action:        action,
		Priority:      hint.Priority,
		Target:        hint.Target,
		EditorSection: hint.EditorSection,
		EditorFocus:   append([]string(nil), hint.EditorFocus...),
		RevisionPath:  hint.RevisionPath,
		Description:   hint.Description,
		FieldPaths:    append([]string(nil), fieldPaths...),
		Patch:         artifacts.patch,
		Skeleton:      artifacts.skeleton,
		Revision:      artifacts.request,
		Validation:    artifacts.validation,
	}
}

func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {
	spec := listingworkspace.BuildReadinessGuidanceSpec(key, warningOnly)
	if spec == nil || spec.Reason == nil {
		return sheinReadinessGuidance{}
	}

	guidance := sheinReadinessGuidance{
		reason: buildSheinReadinessReason(spec.Reason),
	}
	patch := buildSheinReadinessPatchPayload(pkg, key)
	for _, hint := range spec.Hints {
		guidance.repairHints = append(guidance.repairHints, buildSheinReadinessRepairHint(
			pkg,
			suggestedAction,
			fieldPaths,
			hint,
			patch,
		))
	}
	return guidance
}

func cloneSheinReadinessReason(reason *SheinReadinessReason) *SheinReadinessReason {
	if reason == nil {
		return nil
	}
	cloned := *reason
	return &cloned
}

func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinRepairHint, 0, len(items))
	for _, item := range items {
		artifacts := cloneSheinRepairArtifacts(item.Patch, item.Skeleton, item.Revision, item.Validation)
		cloned = append(cloned, SheinRepairHint{
			Action:        item.Action,
			Priority:      item.Priority,
			Target:        item.Target,
			EditorSection: item.EditorSection,
			EditorFocus:   append([]string(nil), item.EditorFocus...),
			RevisionPath:  item.RevisionPath,
			Description:   item.Description,
			FieldPaths:    append([]string(nil), item.FieldPaths...),
			Patch:         artifacts.patch,
			Skeleton:      artifacts.skeleton,
			Revision:      artifacts.request,
			Validation:    artifacts.validation,
		})
	}
	return cloned
}

func sheinHasAnySKU(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 0 {
			return true
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if len(skc.SKUList) > 0 {
				return true
			}
		}
	}
	return false
}
