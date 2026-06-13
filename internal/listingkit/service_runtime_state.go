package listingkit

type serviceRuntimeState struct {
	taskSubmitter TaskSubmitter

	sheinPublishWorkflowClient     SheinPublishWorkflowClient
	sheinPublishWorkflowEnabled    bool
	standardProductWorkflowClient  StandardProductWorkflowClient
	standardProductWorkflowEnabled bool
	platformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled   bool
}
