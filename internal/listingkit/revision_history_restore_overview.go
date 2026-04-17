package listingkit

type RevisionHistoryRestoreOverview struct {
	Status        string   `json:"status,omitempty"`
	Headline      string   `json:"headline,omitempty"`
	Subheadline   string   `json:"subheadline,omitempty"`
	PrimaryAction string   `json:"primary_action,omitempty"`
	NextActions   []string `json:"next_actions,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

func buildRevisionHistoryRestoreOverview(record *ListingKitRevisionRecord, safety *RevisionHistoryRestoreSafety, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreOverview {
	if record == nil && safety == nil && comparePreview == nil {
		return nil
	}

	data := buildRevisionHistoryRestoreOverviewData(record, safety, comparePreview)
	return &RevisionHistoryRestoreOverview{
		Status:        data.Status,
		Headline:      data.Title,
		Subheadline:   data.Subtitle,
		PrimaryAction: data.PrimaryAction,
		NextActions:   data.NextActions,
		Highlights:    data.Highlights,
	}
}
