package listingkit

type taskDependencies struct {
	sdsLoginStatusProvider       SDSLoginStatusProvider
	taskSubmitter                TaskSubmitter
	generationUsage              GenerationUsageSettlement
	requestDefaults              generateRequestDefaults
	standardWorkflowClient       StandardProductWorkflowClient
	standardWorkflowEnabled      bool
	platformAdaptWorkflowClient  PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled bool
}
