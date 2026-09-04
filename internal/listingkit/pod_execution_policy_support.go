package listingkit

import sdspod "task-processor/internal/sds/adapter/product_source"

func podSubmissionBlocked(pod *PodExecutionSummary) bool {
	return sdspod.SubmissionBlocked(podExecutionPolicyState(pod))
}

func podReadinessMessage(pod *PodExecutionSummary) string {
	return sdspod.ReadinessMessage(podExecutionPolicyState(pod))
}
