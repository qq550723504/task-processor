package workspace

type HistoryRestoreRecommendedView struct {
	View   string
	Reason string
}

type HistoryRestorePresentationData struct {
	Status           string
	Headline         string
	Subheadline      string
	PrimaryAction    string
	NextActions      []string
	Highlights       []string
	Title            string
	Description      string
	ConfirmLabel     string
	WarningTitle     string
	WarningSummaries []string
	RecommendedView  *HistoryRestoreRecommendedView
}

func BuildHistoryRestorePresentationData(
	record *HistoryRestoreRecordInput,
	context *HistoryRestoreContext,
	safety *HistoryRestoreSafety,
	compare *HistoryRestoreCompareInput,
) *HistoryRestorePresentationData {
	overview := BuildHistoryRestoreOverview(record, safety, compare)
	messages := BuildHistoryRestoreMessages(context, safety, overview)
	if overview == nil && messages == nil {
		return nil
	}

	data := &HistoryRestorePresentationData{}
	if overview != nil {
		data.Status = overview.Status
		data.Headline = overview.Headline
		data.Subheadline = overview.Subheadline
		data.PrimaryAction = overview.PrimaryAction
		data.NextActions = append([]string(nil), overview.NextActions...)
		data.Highlights = append([]string(nil), overview.Highlights...)
	}
	if messages != nil {
		data.Title = messages.Title
		data.Description = messages.Description
		data.ConfirmLabel = messages.ConfirmLabel
		data.WarningTitle = messages.WarningTitle
		data.WarningSummaries = append([]string(nil), messages.WarningSummaries...)
	}
	data.RecommendedView = buildHistoryRestoreRecommendedView(safety)
	data.NextActions = uniqueStrings(data.NextActions)
	data.Highlights = uniqueStrings(data.Highlights)
	data.WarningSummaries = uniqueStrings(data.WarningSummaries)
	return data
}

func buildHistoryRestoreRecommendedView(safety *HistoryRestoreSafety) *HistoryRestoreRecommendedView {
	if safety == nil {
		return nil
	}
	switch {
	case !safety.CanRestore:
		return &HistoryRestoreRecommendedView{
			View:   "inspection",
			Reason: "当前恢复条件还不完整，建议先检查阻塞项。",
		}
	case len(safety.RestoreWarnings) > 0:
		return &HistoryRestoreRecommendedView{
			View:   "inspection",
			Reason: "恢复前还有提醒项，建议先确认影响范围。",
		}
	default:
		return &HistoryRestoreRecommendedView{
			View:   "submit",
			Reason: "当前可以直接执行恢复。",
		}
	}
}
