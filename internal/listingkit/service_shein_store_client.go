package listingkit

import (
	"context"

	sheinclient "task-processor/internal/shein/client"
)

func (s *service) resolveSheinStoreInfo(ctx context.Context, task *Task) (*SheinStoreInfo, error) {
	return buildSubmitRuntimeContextResolver(s).resolveStoreInfo(ctx, task)
}

func (s *service) newSheinAPIClient(ctx context.Context, task *Task) (*sheinclient.APIClient, int64, error) {
	return buildSubmitRuntimeContextResolver(s).newAPIClient(ctx, task)
}
