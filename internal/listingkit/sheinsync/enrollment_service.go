package sheinsync

import (
	"context"
)

type SheinEnrollmentService interface {
	StartSheinActivityEnrollment(
		ctx context.Context,
		tenantID, storeID int64,
		activityType string,
		activityKey string,
		triggerMode SheinEnrollmentRunTriggerMode,
		candidateIDs ...int64,
	) (*SheinActivityEnrollmentRunRecord, error)
	ExecuteSheinActivityEnrollment(
		ctx context.Context,
		tenantID, storeID int64,
		activityType string,
		activityKey string,
		triggerMode SheinEnrollmentRunTriggerMode,
		candidateIDs ...int64,
	) (*SheinActivityEnrollmentRunRecord, error)
	ExecuteAutoSheinActivityEnrollment(ctx context.Context, tenantID, storeID int64, activityType string, activityKey string) (*SheinActivityEnrollmentRunRecord, error)
}

type SheinEnrollmentRepository interface {
	ListCandidates(ctx context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error)
	SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error
	ListSyncedProducts(ctx context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error)
	CreateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	UpdateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error
	SaveEnrollmentItems(ctx context.Context, items []*SheinActivityEnrollmentItemRecord) error
}

type sheinEnrollmentService struct {
	repo    SheinEnrollmentRepository
	adapter SheinActivityAdapter
}

func NewSheinEnrollmentService(repo SheinEnrollmentRepository, adapter SheinActivityAdapter) SheinEnrollmentService {
	return &sheinEnrollmentService{
		repo:    repo,
		adapter: adapter,
	}
}

const maxSheinEnrollmentCandidatePageSize = 200
