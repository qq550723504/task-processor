package listingkit

import "context"

func (s *service) ClearSheinResolutionCache(ctx context.Context, taskID string, kind string) (*SheinResolutionCacheClearResult, error) {
	return s.sheinAdminOrDefault().ClearSheinResolutionCache(ctx, taskID, kind)
}
