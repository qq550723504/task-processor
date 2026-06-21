package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) retrySheinSensitiveWordSubmit(ctx context.Context, taskID string, pkg *SheinPackage, action string, requestID string, productAPI sheinproduct.ProductAPI, submitProduct *sheinproduct.Product, response *sheinpub.SubmissionResponse, responseErr error) (*sheinpub.SubmissionResponse, error, bool) {
	return sheinpub.RetrySensitiveWordSubmit(ctx, taskID, pkg, action, requestID, productAPI, submitProduct, response, responseErr, s.taskSubmissionExecutionOrDefault().executeSheinSubmitRemote)
}
