package listingkit

type Service interface {
	TaskLifecycleService
	TaskRecoveryService
	TaskRequeueService
	StudioBatchRunService
	StudioMediaService
	StoreAdminService
	InternalListingKitService
}
