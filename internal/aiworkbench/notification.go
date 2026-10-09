package aiworkbench

import "context"

type PlanningNotice struct {
	Command             PlanningCommand
	Confirmed, Archived bool
}
type PlanningNoticeReader interface {
	ListPlanningNotices(context.Context, Scope, string, int) ([]PlanningNotice, string, error)
}
