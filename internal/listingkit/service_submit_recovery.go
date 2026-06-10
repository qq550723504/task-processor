package listingkit

import (
	"context"
	"time"

	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	return s.taskSubmissionRecoveryOrDefault().mutateTaskResult(ctx, taskID, mutate)
}

func (s *service) RefreshSubmissionStatus(ctx context.Context, taskID string) (*ListingKitPreview, error) {
	return s.taskSubmissionRefreshOrDefault().RefreshSubmissionStatus(ctx, taskID)
}

func (s *service) resolveSheinSubmitRemoteStatus(productAPI sheinproduct.ProductAPI, otherAPI sheinother.OtherAPI, action, requestID string, lookupCodes []string, spuName string, defaultConfirmed bool, fallbackMessage string, startedAt time.Time, taskID string) (*sheinRemoteConfirmation, error) {
	return s.taskSubmissionRecoveryOrDefault().resolveSheinSubmitRemoteStatus(productAPI, otherAPI, action, requestID, lookupCodes, spuName, defaultConfirmed, fallbackMessage, startedAt, taskID)
}
