package sheinsync

import "context"

type SheinSyncScheduler struct {
	syncService SheinSyncService
}

func NewSheinSyncScheduler(syncService SheinSyncService) *SheinSyncScheduler {
	return &SheinSyncScheduler{syncService: syncService}
}

func (s *SheinSyncScheduler) Run(ctx context.Context, tenantID, storeID int64) (*SheinSyncJobRecord, error) {
	if s == nil || s.syncService == nil {
		return nil, nil
	}
	return s.syncService.SyncSheinOnShelfProducts(ctx, tenantID, storeID, SheinSyncTriggerModeSchedule)
}
