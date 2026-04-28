package addresscopy

import "task-processor/internal/shein/api/warehouse"

type CopyRequest struct {
	SourceStoreID int64
	TargetStoreID int64
	DryRun        bool
}

type CopyResult struct {
	SourceStoreID int64
	TargetStoreID int64
	Total         int
	Copied        int
	Skipped       int
	Failed        int
	Items         []CopyItemResult
}

type CopyItemResult struct {
	Address       *warehouse.StoreAddress
	Action        string
	Reason        string
	WarehouseName string
}
