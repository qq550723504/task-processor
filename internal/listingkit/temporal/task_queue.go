package temporal

const (
	// TaskQueueSheinSubmitPublish is the Temporal task queue for the SHEIN
	// submit publish workflow PoC.
	TaskQueueSheinSubmitPublish = "listingkit-shein-submit-publish"
	// SheinPublishQueryCurrentState is the query name for reading workflow state.
	SheinPublishQueryCurrentState = "current_state"
	// SheinPublishSignalRetry is the signal name for retrying a failed publish.
	SheinPublishSignalRetry = "retry"
)

// WorkflowIDForSheinPublish builds the workflow ID for a SHEIN publish task.
func WorkflowIDForSheinPublish(taskID string) string {
	return "shein-submit:" + taskID + ":publish"
}
