package shein

import "strings"

type Guidance[R any, H any] struct {
	Reason      *R
	RepairHints []H
}

type ReadinessCheckSpec struct {
	Key             string
	Label           string
	OK              bool
	Message         string
	FieldPaths      []string
	SuggestedAction string
	WarningOnly     bool
}

type SubmitReadiness[R any, H any] struct {
	Ready         bool                   `json:"ready"`
	Status        string                 `json:"status,omitempty"`
	Summary       []string               `json:"summary,omitempty"`
	BlockingItems []ReadinessItem[R, H]  `json:"blocking_items,omitempty"`
	WarningItems  []ReadinessItem[R, H]  `json:"warning_items,omitempty"`
	Checks        []ReadinessCheck[R, H] `json:"checks,omitempty"`
}

type ReadinessItem[R any, H any] struct {
	Key             string   `json:"key,omitempty"`
	Label           string   `json:"label,omitempty"`
	Message         string   `json:"message,omitempty"`
	FieldPaths      []string `json:"field_paths,omitempty"`
	SuggestedAction string   `json:"suggested_action,omitempty"`
	Reason          *R       `json:"reason,omitempty"`
	RepairHints     []H      `json:"repair_hints,omitempty"`
}

type ReadinessCheck[R any, H any] struct {
	Key             string   `json:"key,omitempty"`
	Label           string   `json:"label,omitempty"`
	Status          string   `json:"status,omitempty"`
	Message         string   `json:"message,omitempty"`
	FieldPaths      []string `json:"field_paths,omitempty"`
	SuggestedAction string   `json:"suggested_action,omitempty"`
	Reason          *R       `json:"reason,omitempty"`
	RepairHints     []H      `json:"repair_hints,omitempty"`
}

type SubmitChecklist[R any, H any] struct {
	Required    []ChecklistGroupItem[R, H] `json:"required,omitempty"`
	Recommended []ChecklistGroupItem[R, H] `json:"recommended,omitempty"`
	Optional    []ChecklistGroupItem[R, H] `json:"optional,omitempty"`
}

type ChecklistGroupItem[R any, H any] struct {
	Key             string   `json:"key,omitempty"`
	Label           string   `json:"label,omitempty"`
	Status          string   `json:"status,omitempty"`
	Message         string   `json:"message,omitempty"`
	FieldPaths      []string `json:"field_paths,omitempty"`
	SuggestedAction string   `json:"suggested_action,omitempty"`
	Reason          *R       `json:"reason,omitempty"`
	RepairHints     []H      `json:"repair_hints,omitempty"`
}

func BuildSubmitReadiness[R any, H any](
	checks []ReadinessCheckSpec,
	guidanceResolver func(ReadinessCheckSpec) Guidance[R, H],
	blockedSummary string,
	warningSummary string,
	readySummary string,
) *SubmitReadiness[R, H] {
	if len(checks) == 0 {
		return nil
	}

	readiness := &SubmitReadiness[R, H]{}
	var blockers []ReadinessItem[R, H]
	var warnings []ReadinessItem[R, H]
	var resolvedChecks []ReadinessCheck[R, H]

	for _, spec := range checks {
		guidance := guidanceResolver(spec)
		status := "ready"
		if !spec.OK && spec.WarningOnly {
			status = "warning"
		}
		if !spec.OK && !spec.WarningOnly {
			status = "blocking"
		}

		check := ReadinessCheck[R, H]{
			Key:             spec.Key,
			Label:           spec.Label,
			Status:          status,
			Message:         spec.Message,
			FieldPaths:      append([]string(nil), spec.FieldPaths...),
			SuggestedAction: spec.SuggestedAction,
			Reason:          guidance.Reason,
			RepairHints:     append([]H(nil), guidance.RepairHints...),
		}
		resolvedChecks = append(resolvedChecks, check)
		if spec.OK {
			continue
		}
		item := ReadinessItem[R, H]{
			Key:             spec.Key,
			Label:           spec.Label,
			Message:         spec.Message,
			FieldPaths:      append([]string(nil), spec.FieldPaths...),
			SuggestedAction: spec.SuggestedAction,
			Reason:          guidance.Reason,
			RepairHints:     append([]H(nil), guidance.RepairHints...),
		}
		if spec.WarningOnly {
			warnings = append(warnings, item)
		} else {
			blockers = append(blockers, item)
		}
	}

	readiness.Checks = resolvedChecks
	readiness.BlockingItems = blockers
	readiness.WarningItems = warnings
	readiness.Ready = len(blockers) == 0
	switch {
	case len(blockers) > 0:
		readiness.Status = "blocked"
		readiness.Summary = append(readiness.Summary, blockedSummary)
	case len(warnings) > 0:
		readiness.Status = "ready_with_warnings"
		readiness.Summary = append(readiness.Summary, warningSummary)
	default:
		readiness.Status = "ready"
		readiness.Summary = append(readiness.Summary, readySummary)
	}
	return readiness
}

func BuildSubmitChecklist[R any, H any](readiness *SubmitReadiness[R, H], groupForCheck func(string) string) *SubmitChecklist[R, H] {
	if readiness == nil {
		return nil
	}

	checklist := &SubmitChecklist[R, H]{}
	for _, check := range readiness.Checks {
		item := ChecklistGroupItem[R, H]{
			Key:             check.Key,
			Label:           check.Label,
			Status:          check.Status,
			Message:         check.Message,
			FieldPaths:      append([]string(nil), check.FieldPaths...),
			SuggestedAction: check.SuggestedAction,
			Reason:          check.Reason,
			RepairHints:     append([]H(nil), check.RepairHints...),
		}
		if source := findReadinessItem(readiness, check.Key); source != nil {
			item.SuggestedAction = source.SuggestedAction
			item.Reason = source.Reason
			item.RepairHints = append([]H(nil), source.RepairHints...)
		}

		switch groupForCheck(check.Key) {
		case "required":
			checklist.Required = append(checklist.Required, item)
		case "recommended":
			checklist.Recommended = append(checklist.Recommended, item)
		default:
			checklist.Optional = append(checklist.Optional, item)
		}
	}

	if len(checklist.Required) == 0 && len(checklist.Recommended) == 0 && len(checklist.Optional) == 0 {
		return nil
	}
	return checklist
}

func ChecklistItemCount[R any, H any](checklist *SubmitChecklist[R, H]) int {
	if checklist == nil {
		return 0
	}
	return len(checklist.Required) + len(checklist.Recommended) + len(checklist.Optional)
}

func SubmitChecklistGroupForKey(key string) string {
	switch key {
	case "category", "category_review", "attributes", "attribute_review", "sale_attributes", "images", "variants":
		return "required"
	case "request_draft", "preview_product":
		return "recommended"
	default:
		return "optional"
	}
}

func FindLabels[R any, H any](items []ReadinessItem[R, H]) []string {
	if len(items) == 0 {
		return nil
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		if item.Label != "" {
			labels = append(labels, item.Label)
		}
	}
	return labels
}

func FindKeys[R any, H any](items []ReadinessItem[R, H]) []string {
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.Key != "" {
			keys = append(keys, item.Key)
		}
	}
	return keys
}

func CloneReadinessItems[R any, H any](items []ReadinessItem[R, H]) []ReadinessItem[R, H] {
	if len(items) == 0 {
		return nil
	}
	out := make([]ReadinessItem[R, H], len(items))
	copy(out, items)
	return out
}

func ToActionItems[R any, H any](items []ReadinessItem[R, H]) []ActionItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]ActionItem, 0, len(items))
	for _, item := range items {
		out = append(out, ActionItem{
			Key:             item.Key,
			SuggestedAction: item.SuggestedAction,
		})
	}
	return out
}

func findReadinessItem[R any, H any](readiness *SubmitReadiness[R, H], key string) *ReadinessItem[R, H] {
	if readiness == nil || key == "" {
		return nil
	}
	for i := range readiness.BlockingItems {
		if readiness.BlockingItems[i].Key == key {
			return &readiness.BlockingItems[i]
		}
	}
	for i := range readiness.WarningItems {
		if readiness.WarningItems[i].Key == key {
			return &readiness.WarningItems[i]
		}
	}
	return nil
}

func JoinReadinessLabels[R any, H any](items []ReadinessItem[R, H], sep string) string {
	return strings.Join(FindLabels(items), sep)
}
