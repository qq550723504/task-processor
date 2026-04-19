package listingkit

import "strings"

type RevisionHistoryDetailQuery struct {
	CompareTo string `form:"compare_to"`
}

func normalizeRevisionHistoryDetailQuery(query *RevisionHistoryDetailQuery) RevisionHistoryDetailQuery {
	if query == nil {
		return RevisionHistoryDetailQuery{}
	}
	return RevisionHistoryDetailQuery{
		CompareTo: strings.TrimSpace(query.CompareTo),
	}
}
