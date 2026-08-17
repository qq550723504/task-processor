package listingkit

type taskDependencies struct {
	sdsLoginStatusProvider       SDSLoginStatusProvider
	taskSubmitter                TaskSubmitter
	generationUsage              GenerationUsageSettlement
	generationUsageAdmission     GenerationUsageAdmission
	standardWorkflowClient       StandardProductWorkflowClient
	standardWorkflowEnabled      bool
	platformAdaptWorkflowClient  PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled bool
}
