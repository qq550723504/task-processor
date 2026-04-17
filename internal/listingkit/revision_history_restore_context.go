package listingkit

type RevisionHistoryRestoreContext struct {
	SourceRevisionID string `json:"source_revision_id,omitempty"`
	SourceActionType string `json:"source_action_type,omitempty"`
	SourceHeadline   string `json:"source_headline,omitempty"`
	TargetRevisionID string `json:"target_revision_id,omitempty"`
	TargetLabel      string `json:"target_label,omitempty"`
	CompareMode      string `json:"compare_mode,omitempty"`
	ExecutionMode    string `json:"execution_mode,omitempty"`
	RestoreReason    string `json:"restore_reason,omitempty"`
	RestorePlatform  string `json:"restore_platform,omitempty"`
}

func buildRevisionHistoryRestoreContext(record *ListingKitRevisionRecord, payload *ApplyRevisionRequest, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreContext {
	if record == nil && payload == nil && comparePreview == nil {
		return nil
	}

	context := &RevisionHistoryRestoreContext{
		ExecutionMode: "restore_from_revision_id",
	}
	if record != nil {
		context.SourceRevisionID = record.RevisionID
		context.SourceActionType = record.ActionType
		if record.Timeline != nil {
			context.SourceHeadline = record.Timeline.Headline
		}
	}
	if payload != nil {
		context.RestorePlatform = payload.Platform
		context.RestoreReason = payload.Reason
	}
	if comparePreview != nil {
		context.CompareMode = comparePreview.CompareTo
		context.TargetRevisionID = comparePreview.CompareRevisionID
		context.TargetLabel = comparePreview.RelationLabel
	}
	if context.TargetRevisionID == "" {
		context.TargetRevisionID = "current"
	}
	if context.TargetLabel == "" {
		context.TargetLabel = "当前版本"
	}
	return context
}
