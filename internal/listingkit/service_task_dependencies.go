package listingkit

type taskDependencies struct {
	sdsLoginStatusProvider       SDSLoginStatusProvider
	taskSubmitter                TaskSubmitter
	requestDefaults              generateRequestDefaults
	standardWorkflowClient       StandardProductWorkflowClient
	standardWorkflowEnabled      bool
	platformAdaptWorkflowClient  PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled bool
}
