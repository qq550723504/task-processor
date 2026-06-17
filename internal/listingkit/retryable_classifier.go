package listingkit

import submissiondomain "task-processor/internal/listing/submission"

func classifyRetryableTaskFailure(err error) (*RetryableBlock, bool) {
	block, ok := submissiondomain.ClassifyRetryableFailure(err, submissiondomain.RetryableRecoveryScopeTask)
	if !ok {
		return nil, false
	}
	return adaptSubmissionRetryableBlock(block), true
}
