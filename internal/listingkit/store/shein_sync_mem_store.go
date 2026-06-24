package store

import (
	"sync"

	"task-processor/internal/listingkit"
)

type MemSheinSyncRepository struct {
	mu              sync.RWMutex
	nextProductID   int64
	nextJobID       int64
	nextCandidateID int64
	nextRunID       int64
	products        map[string]listingkit.SheinSyncedProductRecord
	syncJobs        map[int64]listingkit.SheinSyncJobRecord
	candidates      map[string]listingkit.SheinActivityCandidateRecord
	enrollmentRuns  map[int64]listingkit.SheinActivityEnrollmentRunRecord
	enrollmentItems map[string]listingkit.SheinActivityEnrollmentItemRecord
	sdsCostGroups   map[string]listingkit.SheinSDSCostGroupRecord
}

func NewMemSheinSyncRepository() listingkit.SheinSyncRepository {
	return &MemSheinSyncRepository{
		nextProductID:   1,
		nextJobID:       1,
		nextCandidateID: 1,
		nextRunID:       1,
		products:        make(map[string]listingkit.SheinSyncedProductRecord),
		syncJobs:        make(map[int64]listingkit.SheinSyncJobRecord),
		candidates:      make(map[string]listingkit.SheinActivityCandidateRecord),
		enrollmentRuns:  make(map[int64]listingkit.SheinActivityEnrollmentRunRecord),
		enrollmentItems: make(map[string]listingkit.SheinActivityEnrollmentItemRecord),
		sdsCostGroups:   make(map[string]listingkit.SheinSDSCostGroupRecord),
	}
}
