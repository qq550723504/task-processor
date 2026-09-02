package listingkit

type Service interface {
	TaskLifecycleService
	TaskRecoveryService
	TaskRequeueService
	UploadedImageService
	StoreAdminService
	InternalListingKitService
}
