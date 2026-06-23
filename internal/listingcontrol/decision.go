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
	TaskID    int64  `json:"taskId"`
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	Action    string `json:"action"`
	Queue     string `json:"queue,omitempty"`
	Reason    string `json:"reason,omitempty"`
	OwnerNode string `json:"ownerNode,omitempty"`
	Capacity  int    `json:"capacity"`
	Queued    int64  `json:"queued"`
}

type DispatchSummary struct {
	Candidates int                `json:"candidates"`
	Dispatched int                `json:"dispatched"`
	Skipped    int                `json:"skipped"`
	Failed     int                `json:"failed"`
	Decisions  []DispatchDecision `json:"decisions,omitempty"`
}
