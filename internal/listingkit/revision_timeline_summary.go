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
		if headline := buildReasonSpecificRevisionHeadline(record); headline != "" {
			return headline
		}
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
	if relation := buildReasonSpecificRevisionRelationText(record); relation != "" {
		return relation
	}
	if strings.TrimSpace(record.RestoredFromRevisionID) != "" {
		return "恢复自 " + strings.TrimSpace(record.RestoredFromRevisionID)
	}
	if record.AppliedChanges != nil && record.AppliedChanges.ChangeCount > 0 {
		return "更新了字段"
	}
	return ""
}

func buildReasonSpecificRevisionHeadline(record ListingKitRevisionRecord) string {
	reason := strings.TrimSpace(record.Reason)
	switch reason {
	case "Refresh SHEIN category":
		return "刷新 SHEIN 类目模板"
	case "Regenerate SHEIN attributes":
		return "刷新 SHEIN 普通属性"
	case "Regenerate SHEIN sale attributes":
		return "刷新 SHEIN 销售属性"
	case "Apply suggested SHEIN category":
		return "应用建议 SHEIN 类目"
	case "Confirm current SHEIN category":
		return "确认当前 SHEIN 类目"
	}
	return ""
}

func buildReasonSpecificRevisionRelationText(record ListingKitRevisionRecord) string {
	reason := strings.TrimSpace(record.Reason)
	switch reason {
	case "Refresh SHEIN category", "Apply suggested SHEIN category", "Confirm current SHEIN category":
		return "将重算类目 / 普通属性 / 销售属性"
	case "Regenerate SHEIN attributes":
		return "将按最新模板重新生成普通属性"
	case "Regenerate SHEIN sale attributes":
		return "将按最新模板重新生成销售属性"
	}
	return ""
}
