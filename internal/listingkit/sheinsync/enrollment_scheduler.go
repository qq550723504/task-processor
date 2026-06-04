package sheinsync

import "context"

type SheinEnrollmentScheduler struct {
	syncService       SheinSyncService
	candidateService  SheinCandidateService
	enrollmentService SheinEnrollmentService
}

func NewSheinEnrollmentScheduler(syncService SheinSyncService, candidateService SheinCandidateService, enrollmentService SheinEnrollmentService) *SheinEnrollmentScheduler {
	return &SheinEnrollmentScheduler{
		syncService:       syncService,
		candidateService:  candidateService,
		enrollmentService: enrollmentService,
	}
}

func (s *SheinEnrollmentScheduler) Run(ctx context.Context, tenantID, storeID int64, activityType string) error {
	if _, err := s.syncService.SyncSheinOnShelfProducts(ctx, tenantID, storeID, SheinSyncTriggerModeSchedule); err != nil {
		return err
	}
	if _, err := s.candidateService.RefreshCandidates(ctx, tenantID, storeID, activityType); err != nil {
		return err
	}
	_, err := s.enrollmentService.ExecuteAutoSheinActivityEnrollment(ctx, tenantID, storeID, activityType, "")
	return err
}
