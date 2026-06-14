package workspace

type HistoryNavigation struct {
	PrevRevisionID string `json:"prev_revision_id,omitempty"`
	NextRevisionID string `json:"next_revision_id,omitempty"`
}

type RestorePreviewCoreData[Req any, Ctx any, Safety any, Compare any] struct {
	Draft           *EditorRevisionSkeleton `json:"draft,omitempty"`
	RevisionPayload *Req                    `json:"revision_payload,omitempty"`
	Context         *Ctx                    `json:"context,omitempty"`
	Safety          *Safety                 `json:"safety,omitempty"`
	Compare         *Compare                `json:"compare,omitempty"`
}

type RestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any] struct {
	Core         *RestorePreviewCoreData[Req, Ctx, Safety, Compare] `json:"core,omitempty"`
	Presentation *Pres                                              `json:"presentation,omitempty"`
}

func BuildHistoryNavigation(prevRevisionID, nextRevisionID string) *HistoryNavigation {
	if prevRevisionID == "" && nextRevisionID == "" {
		return nil
	}
	return &HistoryNavigation{
		PrevRevisionID: prevRevisionID,
		NextRevisionID: nextRevisionID,
	}
}
