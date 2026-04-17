package shein

type HistoryRestoreDetailData[Req any, Compare any] struct {
	RevisionPayload *Req                    `json:"revision_payload,omitempty"`
	Context         *HistoryRestoreContext  `json:"context,omitempty"`
	Safety          *HistoryRestoreSafety   `json:"safety,omitempty"`
	Overview        *HistoryRestoreOverview `json:"overview,omitempty"`
	Messages        *HistoryRestoreMessages `json:"messages,omitempty"`
	Compare         *Compare                `json:"compare,omitempty"`
}

func BuildHistoryRestoreDetailData[Req any, Compare any](
	record *HistoryRestoreRecordInput,
	state *HistoryRestoreStateInput,
	draft *EditorRevisionSkeleton,
	revisionPayload *Req,
	executionMode string,
	platform string,
	reason string,
	compareInput *HistoryRestoreCompareInput,
	compareValue *Compare,
) *HistoryRestoreDetailData[Req, Compare] {
	if record == nil && state == nil && draft == nil && revisionPayload == nil && compareInput == nil && compareValue == nil {
		return nil
	}

	context := BuildHistoryRestoreContext(record, executionMode, platform, reason, compareInput)
	safety := BuildHistoryRestoreSafety(state, record, draft, compareInput)
	overview := BuildHistoryRestoreOverview(record, safety, compareInput)
	messages := BuildHistoryRestoreMessages(context, safety, overview)

	return &HistoryRestoreDetailData[Req, Compare]{
		RevisionPayload: revisionPayload,
		Context:         context,
		Safety:          safety,
		Overview:        overview,
		Messages:        messages,
		Compare:         compareValue,
	}
}
