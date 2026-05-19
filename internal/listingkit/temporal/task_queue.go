package temporal

const (
	// TaskQueueSheinSubmitPublish is the Temporal task queue for the SHEIN
	// submit publish workflow PoC.
	TaskQueueSheinSubmitPublish = "listingkit-shein-submit-publish"
	// TaskQueueStandardProduct is the Temporal task queue for the standard
	// product generation layer.
	TaskQueueStandardProduct = TaskQueueSheinSubmitPublish
	// TaskQueuePlatformAdapt is the Temporal task queue for the platform
	// adaptation layer.
	TaskQueuePlatformAdapt = TaskQueueSheinSubmitPublish
	// SheinPublishQueryCurrentState is the query name for reading workflow state.
	SheinPublishQueryCurrentState = "current_state"
	// SheinPublishSignalRetry is the signal name for retrying a failed publish.
	SheinPublishSignalRetry = "retry"
)

// WorkflowIDForSheinPublish builds the workflow ID for a SHEIN publish task.
func WorkflowIDForSheinPublish(taskID string) string {
	return "shein-submit:" + taskID + ":publish"
}

func WorkflowIDForStandardProduct(taskID string) string {
	return "listingkit-standard:" + taskID
}

func WorkflowIDForPlatformAdapt(taskID, platform string) string {
	return "listingkit-platform:" + platform + ":" + taskID
}
