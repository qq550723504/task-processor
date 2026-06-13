package preview

const DefaultMaxRevisionHistoryRecords = 20

type RevisionHistoryMeta struct {
	TotalRecords    int  `json:"total_records"`
	ReturnedRecords int  `json:"returned_records"`
	HasMore         bool `json:"has_more"`
	IsTruncated     bool `json:"is_truncated"`
	MaxRecords      int  `json:"max_records"`
}

type RevisionHistoryMetaInput struct {
	TotalRecords    int
	ReturnedRecords int
	MaxRecords      int
}

func BuildRevisionHistoryMeta(input RevisionHistoryMetaInput) *RevisionHistoryMeta {
	maxRecords := input.MaxRecords
	if maxRecords <= 0 {
		maxRecords = DefaultMaxRevisionHistoryRecords
	}
	total := input.TotalRecords
	if total < input.ReturnedRecords {
		total = input.ReturnedRecords
	}
	return &RevisionHistoryMeta{
		TotalRecords:    total,
		ReturnedRecords: input.ReturnedRecords,
		HasMore:         total > input.ReturnedRecords,
		IsTruncated:     total > input.ReturnedRecords,
		MaxRecords:      maxRecords,
	}
}
