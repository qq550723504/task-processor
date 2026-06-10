package listingkit

import "context"

func (s *service) UpdateSheinFinalDraft(ctx context.Context, taskID string, req *SheinFinalDraftUpdateRequest) (*ListingKitPreview, error) {
	return s.sheinAdminOrDefault().UpdateSheinFinalDraft(ctx, taskID, req)
}
