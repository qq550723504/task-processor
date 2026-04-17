package listingkit

type SheinSubmitChecklist struct {
	Required    []SheinChecklistGroupItem `json:"required,omitempty"`
	Recommended []SheinChecklistGroupItem `json:"recommended,omitempty"`
	Optional    []SheinChecklistGroupItem `json:"optional,omitempty"`
}

type SheinChecklistGroupItem struct {
	Key             string                `json:"key,omitempty"`
	Label           string                `json:"label,omitempty"`
	Status          string                `json:"status,omitempty"`
	Message         string                `json:"message,omitempty"`
	FieldPaths      []string              `json:"field_paths,omitempty"`
	SuggestedAction string                `json:"suggested_action,omitempty"`
	Reason          *SheinReadinessReason `json:"reason,omitempty"`
	RepairHints     []SheinRepairHint     `json:"repair_hints,omitempty"`
}

func buildSheinSubmitChecklist(readiness *SheinSubmitReadiness) *SheinSubmitChecklist {
	if readiness == nil {
		return nil
	}

	checklist := &SheinSubmitChecklist{}
	for _, check := range readiness.Checks {
		item := SheinChecklistGroupItem{
			Key:             check.Key,
			Label:           check.Label,
			Status:          check.Status,
			Message:         check.Message,
			FieldPaths:      append([]string(nil), check.FieldPaths...),
			SuggestedAction: check.SuggestedAction,
			Reason:          cloneSheinReadinessReason(check.Reason),
			RepairHints:     cloneSheinRepairHints(check.RepairHints),
		}
		if source := findReadinessItem(readiness, check.Key); source != nil {
			item.SuggestedAction = source.SuggestedAction
			item.Reason = cloneSheinReadinessReason(source.Reason)
			item.RepairHints = cloneSheinRepairHints(source.RepairHints)
		}

		switch checklistGroupForCheck(check.Key) {
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

func checklistGroupForCheck(key string) string {
	switch key {
	case "category", "attributes", "sale_attributes", "images", "variants":
		return "required"
	case "request_draft", "preview_product":
		return "recommended"
	default:
		return "optional"
	}
}

func findReadinessItem(readiness *SheinSubmitReadiness, key string) *SheinReadinessItem {
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
