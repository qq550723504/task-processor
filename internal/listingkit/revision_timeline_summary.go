package listingkit

import "strings"

func withRevisionTimelineSummary(record ListingKitRevisionRecord) ListingKitRevisionRecord {
	record.Timeline = buildRevisionTimelineSummary(record)
	return record
}

func buildRevisionTimelineSummary(record ListingKitRevisionRecord) *ListingKitRevisionTimelineSummary {
	platform := strings.ToLower(strings.TrimSpace(record.Platform))
	if platform == "" && record.ActionType == "" && record.RestoredFromRevisionID == "" && record.AppliedChanges == nil {
		return nil
	}

	summary := &ListingKitRevisionTimelineSummary{
		Headline:     buildRevisionTimelineHeadline(record, platform),
		Badge:        buildRevisionTimelineBadge(record),
		RelationText: buildRevisionTimelineRelationText(record),
	}
	if record.AppliedChanges != nil {
		summary.ChangeCount = record.AppliedChanges.ChangeCount
	}
	return summary
}

func buildRevisionTimelineHeadline(record ListingKitRevisionRecord, platform string) string {
	switch record.ActionType {
	case RevisionActionTypeRestore:
		return "恢复历史版本"
	case RevisionActionTypeEdit:
		switch platform {
		case "shein":
			return "更新 SHEIN 资料"
		case "amazon":
			return "更新 Amazon 资料"
		case "temu":
			return "更新 TEMU 资料"
		case "walmart":
			return "更新 Walmart 资料"
		}
	}
	if platform != "" {
		return "更新资料"
	}
	return ""
}

func buildRevisionTimelineBadge(record ListingKitRevisionRecord) string {
	switch record.ActionType {
	case RevisionActionTypeRestore:
		return "回滚"
	case RevisionActionTypeEdit:
		return "编辑"
	default:
		return ""
	}
}

func buildRevisionTimelineRelationText(record ListingKitRevisionRecord) string {
	if strings.TrimSpace(record.RestoredFromRevisionID) != "" {
		return "恢复自 " + strings.TrimSpace(record.RestoredFromRevisionID)
	}
	if record.AppliedChanges != nil && record.AppliedChanges.ChangeCount > 0 {
		return "更新了字段"
	}
	return ""
}
