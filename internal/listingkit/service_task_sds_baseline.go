package listingkit

import "context"

func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {
	return s.warmSDSBaseline(ctx, req)
}
