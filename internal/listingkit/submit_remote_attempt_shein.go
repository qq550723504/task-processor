package listingkit

import (
	"context"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmitRemoteAttemptResult struct {
	response *sheinpub.SubmissionResponse
	err      error
	snapshot *sheinpub.SubmitSnapshot
}

type sheinSubmitRemoteRetryFunc func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)

func executeSheinSubmitRemoteAttempt(
	ctx context.Context,
	taskID string,
	pkg *SheinPackage,
	action string,
	requestID string,
	productAPI sheinproduct.ProductAPI,
	submitProduct *sheinproduct.Product,
	executeRemote func(sheinproduct.ProductAPI, string, *sheinproduct.Product) (*sheinpub.SubmissionResponse, error),
	retrySubmit sheinSubmitRemoteRetryFunc,
) sheinSubmitRemoteAttemptResult {
	response, responseErr := executeRemote(productAPI, action, submitProduct)
	if responseErr == nil {
		responseErr = submission.BuildResponseError(action, response)
	}

	snapshot := sheinpub.BuildSubmitSnapshot(submitProduct)
	if retrySubmit != nil {
		if retryResponse, retryErr, retried := retrySubmit(ctx, taskID, pkg, action, requestID, productAPI, submitProduct, response, responseErr); retried {
			response = retryResponse
			responseErr = retryErr
			snapshot = sheinpub.BuildSubmitSnapshot(submitProduct)
			setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
		}
	}

	return sheinSubmitRemoteAttemptResult{
		response: response,
		err:      responseErr,
		snapshot: snapshot,
	}
}
