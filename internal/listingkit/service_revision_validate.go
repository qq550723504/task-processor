package listingkit

import (
	"context"
)

func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {
	return s.taskRevisionOrDefault().ValidateTaskRevision(ctx, taskID, req)
}
