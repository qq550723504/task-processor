package temporal

import (
	"os"
	"strings"
)

const (
	// EnvTaskQueue overrides the ListingKit Temporal task queue. It is mainly
	// used by local API processes so their workers do not race remote workers.
	EnvTaskQueue = "LISTINGKIT_TEMPORAL_TASK_QUEUE"
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

func TaskQueueSheinSubmitPublishName() string {
	return configuredTaskQueue(TaskQueueSheinSubmitPublish)
}

func TaskQueueStandardProductName() string {
	return configuredTaskQueue(TaskQueueStandardProduct)
}

func TaskQueuePlatformAdaptName() string {
	return configuredTaskQueue(TaskQueuePlatformAdapt)
}

func configuredTaskQueue(fallback string) string {
	if value := strings.TrimSpace(os.Getenv(EnvTaskQueue)); value != "" {
		return value
	}
	return fallback
}

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
