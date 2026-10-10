package operationscockpit

import "context"

type StoreReference struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Region   string `json:"region"`
	Status   string `json:"status"`
}

type Repository interface {
	Execute(context.Context, Command) (Receipt, error)
	Snapshot(context.Context, Scope, Query) (Snapshot, error)
	Goal(context.Context, Scope) (GoalVersion, error)
	HeadMetadata(context.Context, Scope) (HeadMetadata, error)
	Fact(context.Context, Scope, string) (Record, error)
	Facts(context.Context, Scope, string, int) ([]Record, error)
	FactHistory(context.Context, Scope, string, int64) ([]Record, error)
	GoalHistory(context.Context, Scope, int64) ([]GoalVersion, error)
}
