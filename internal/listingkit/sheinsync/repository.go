package sheinsync

import (
	"context"
	"time"
)

type SheinSyncedProductQuery struct {
	TenantID int64
	StoreID  int64
	SKCName  string
	IsActive *bool
	Page     int
	PageSize int
}

type SheinSDSCostGroupQuery struct {
	TenantID  int64
	StoreID   int64
	GroupKeys []string
	Page      int
	PageSize  int
}

type SheinSourceSDSCostGroupQuery struct {
	TenantID int64
	StoreID  int64
	Page     int
	PageSize int
}

type SheinSyncJobQuery struct {
	TenantID    int64
	StoreID     int64
	TriggerMode *SheinSyncTriggerMode
	Status      *SheinSyncJobStatus
	StartedFrom *time.Time
	StartedTo   *time.Time
	Page        int
	PageSize    int
}

type SheinActivityCandidateQuery struct {
	TenantID         int64
	StoreID          int64
	ActivityType     string
	ActivityKey      string
	SKCName          string
	CandidateVersion string
	CandidateIDs     []int64
	Page             int
	PageSize         int
}

type SheinEnrollmentRunQuery struct {
	TenantID     int64
	StoreID      int64
	ActivityType string
	ActivityKey  string
	Status       *SheinEnrollmentRunStatus
	Page         int
	PageSize     int
}

type SheinSyncedProductRepository interface {
	UpsertSyncedProducts(ctx context.Context, records []*SheinSyncedProductRecord) error
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error
	MarkMissingSyncedProductsInactive(ctx context.Context, tenantID, storeID int64, activeSKCNames []string) error
}

type SheinSyncJobRepository interface {
	SaveSyncJob(ctx context.Context, job *SheinSyncJobRecord) error
	ListSyncJobs(ctx context.Context, query *SheinSyncJobQuery) ([]SheinSyncJobRecord, int64, error)
}

type SheinActivityCandidateRepository interface {
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
	SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error
}

type SheinActivityEnrollmentRunRepository interface {
	CreateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	UpdateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	ListEnrollmentRuns(ctx context.Context, query *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error)
}

type SheinActivityEnrollmentItemRepository interface {
	SaveEnrollmentItems(ctx context.Context, items []*SheinActivityEnrollmentItemRecord) error
}

type SheinSyncRepository interface {
	SheinSyncedProductRepository
	SheinSyncJobRepository
	SheinActivityCandidateRepository
	SheinActivityEnrollmentRunRepository
	SheinActivityEnrollmentItemRepository
}
