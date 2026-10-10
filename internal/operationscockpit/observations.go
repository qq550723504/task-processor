package operationscockpit

import "time"

// Saved platform observations keep their own range, timestamp and generation.
// They are never included in manual financial totals or goal evaluation.
type ObservationEvidence struct {
	StoreID    string     `json:"storeId"`
	SyncID     string     `json:"syncId"`
	Status     string     `json:"status"`
	ObservedAt *time.Time `json:"observedAt"`
	Start      *time.Time `json:"start"`
	End        *time.Time `json:"end"`
	ErrorCode  string     `json:"errorCode"`
}
type ObservationProjection struct {
	State       string                `json:"state"`
	Exceptional int                   `json:"exceptional"`
	Complete    bool                  `json:"complete"`
	Sources     []ObservationEvidence `json:"sources"`
	Latest      []ObservationEvidence `json:"latest"`
	ActionPath  string                `json:"actionPath"`
}
