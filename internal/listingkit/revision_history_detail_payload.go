package listingkit

type RevisionHistoryNavigation struct {
	PrevRevisionID string `json:"prev_revision_id,omitempty"`
	NextRevisionID string `json:"next_revision_id,omitempty"`
}

type RevisionRestorePreviewCoreData struct {
	Draft           *SheinEditorRevisionSkeleton   `json:"draft,omitempty"`
	RevisionPayload *ApplyRevisionRequest          `json:"revision_payload,omitempty"`
	Context         *RevisionHistoryRestoreContext `json:"context,omitempty"`
	Safety          *RevisionHistoryRestoreSafety  `json:"safety,omitempty"`
	Compare         *RevisionHistoryComparePreview `json:"compare,omitempty"`
}

type RevisionRestorePreviewPayload struct {
	Core         *RevisionRestorePreviewCoreData  `json:"core,omitempty"`
	Presentation *RevisionInteractionPresentation `json:"presentation,omitempty"`
}

func buildRevisionHistoryNavigation(prevRevisionID, nextRevisionID string) *RevisionHistoryNavigation {
	if prevRevisionID == "" && nextRevisionID == "" {
		return nil
	}
	return &RevisionHistoryNavigation{
		PrevRevisionID: prevRevisionID,
		NextRevisionID: nextRevisionID,
	}
}

func buildRevisionHistoryDetailRestorePayload(
	record *ListingKitRevisionRecord,
	draft *SheinEditorRevisionSkeleton,
	revisionPayload *ApplyRevisionRequest,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	presentation *RevisionInteractionPresentation,
	compare *RevisionHistoryComparePreview,
) *RevisionRestorePreviewPayload {
	if record == nil && draft == nil && revisionPayload == nil && context == nil && safety == nil && presentation == nil && compare == nil {
		return nil
	}
	return &RevisionRestorePreviewPayload{
		Core: &RevisionRestorePreviewCoreData{
			Draft:           draft,
			RevisionPayload: revisionPayload,
			Context:         context,
			Safety:          safety,
			Compare:         compare,
		},
		Presentation: presentation,
	}
}

func buildRevisionRestorePreviewFromDetail(detail *ListingKitRevisionHistoryDetail) *RevisionRestorePreviewPayload {
	if detail == nil || detail.RestorePayload == nil {
		return nil
	}
	compare := detail.RestorePayload.Core.Compare
	if compare == nil && detail.Record != nil {
		compare = &RevisionHistoryComparePreview{
			CompareTo:         "current",
			CompareRevisionID: "current",
			RelationLabel:     "当前版本",
			DiffPreview:       buildSheinRevisionDiffPreviewFromInput(detail.RestorePayload.Core.Draft),
		}
	}
	return &RevisionRestorePreviewPayload{
		Core: &RevisionRestorePreviewCoreData{
			Draft:           detail.RestorePayload.Core.Draft,
			RevisionPayload: detail.RestorePayload.Core.RevisionPayload,
			Context:         detail.RestorePayload.Core.Context,
			Safety:          detail.RestorePayload.Core.Safety,
			Compare:         compare,
		},
		Presentation: detail.RestorePayload.Presentation,
	}
}
