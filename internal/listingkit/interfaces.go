package listingkit

type Service interface {
	TaskLifecycleService
	TaskRecoveryService
	TaskRequeueService
	GenerationTaskService
	StudioBatchRunService
	StudioMediaService
	StoreAdminService
	InternalListingKitService
}
