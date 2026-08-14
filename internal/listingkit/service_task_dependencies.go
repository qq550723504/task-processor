package listingkit

type taskDependencies struct {
	sdsLoginStatusProvider       SDSLoginStatusProvider
	taskSubmitter                TaskSubmitter
	generationUsage              GenerationUsageSettlement
	standardWorkflowClient       StandardProductWorkflowClient
	standardWorkflowEnabled      bool
	platformAdaptWorkflowClient  PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled bool
}
