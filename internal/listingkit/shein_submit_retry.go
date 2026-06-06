package listingkit

import (
	"context"
	"time"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) retrySheinSensitiveWordSubmit(ctx context.Context, taskID string, pkg *SheinPackage, action string, requestID string, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, response *sheinpub.SubmissionResponse, responseErr error) (*sheinpub.SubmissionResponse, error, bool) {
	if action != "publish" || response == nil || responseErr == nil || len(response.ValidationNotes) == 0 {
		return response, responseErr, false
	}
	if !sheinpub.RetrySensitiveWordCleanup(ctx, submitProduct, response.ValidationNotes) {
		return response, responseErr, false
	}

	appendSheinSubmissionEvent(pkg, listingsubmission.BuildPhaseEvent(taskID, action, sheinpub.SubmissionPhaseSubmitRemote, sheinpub.SubmissionStatusRunning, requestID, time.Now(), "检测到敏感词，已自动清理并重试提交", nil))
	retryResponse, retryErr := executeSheinSubmitRemote(productAPI, action, submitProduct)
	if retryErr == nil {
		retryErr = listingsubmission.BuildResponseError(action, retryResponse)
	}
	return retryResponse, retryErr, true
}
