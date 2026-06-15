package api

const (
	defaultSheinSummaryActivityType = "PROMOTION"
	sheinSummaryPageSize            = 100
	sheinSummaryConcurrency         = 8
)

type sheinSummaryQuery struct {
	ActivityType string `form:"activity_type"`
}

type listSheinEnrollmentRunsQuery struct {
	ActivityType string `form:"activity_type"`
	ActivityKey  string `form:"activity_key"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}
