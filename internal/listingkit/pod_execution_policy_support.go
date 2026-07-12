package listingkit

import sdspod "task-processor/internal/product/sourcing/sdspod"

func podSubmissionBlocked(pod *PodExecutionSummary) bool {
	return sdspod.SubmissionBlocked(podExecutionPolicyState(pod))
}

func podReadinessMessage(pod *PodExecutionSummary) string {
	return sdspod.ReadinessMessage(podExecutionPolicyState(pod))
}
