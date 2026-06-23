package listingcontrol

const (
	DispatchActionDispatched = "dispatched"
	DispatchActionSkipped    = "skipped"
	DispatchActionFailed     = "failed"
	DispatchActionDryRun     = "dry_run"

	ReasonClaimConflict = "claim_conflict"
	ReasonStoreUnknown  = "store_unknown"
	ReasonStoreMissing  = "store_missing"
)

type DispatchDecision struct {
	TaskID    int64
	TenantID  int64
	StoreID   int64
	Action    string
	Queue     string
	Reason    string
	OwnerNode string
	Capacity  int
	Queued    int64
}

type DispatchSummary struct {
	Candidates int
	Dispatched int
	Skipped    int
	Failed     int
	Decisions  []DispatchDecision
}
