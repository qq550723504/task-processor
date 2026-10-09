package productsourcing

import (
	"context"
	"task-processor/internal/product/sourcing"
)

func (s *AcquisitionService) NotificationFacts(ctx context.Context, after string, limit int) ([]sourcing.AcquisitionNoticeFact, string, error) {
	scope, e := s.authorizer.Authorize(ctx)
	if e != nil {
		return nil, "", e
	}
	r, ok := s.operations.(sourcing.AcquisitionNoticeReader)
	if !ok {
		return nil, "", sourcing.ErrAcquisitionUnavailable
	}
	operations, next, e := r.ListNoticeOperations(ctx, scope, after, limit)
	if e != nil {
		return nil, "", e
	}
	result := make([]sourcing.AcquisitionNoticeFact, 0, len(operations))
	for _, op := range operations {
		fact := sourcing.AcquisitionNoticeFact{Operation: op}
		if op.State == sourcing.AcquisitionPublished {
			published, e := s.ReadPublished(ctx, op.ID)
			if e != nil {
				return nil, "", e
			}
			fact.Publication = published.Result.Publication
		}
		result = append(result, fact)
	}
	return result, next, nil
}
func (s *BrowserAcquisitionService) NotificationFacts(ctx context.Context, after string, limit int) ([]sourcing.AcquisitionNoticeFact, string, error) {
	return s.core.NotificationFacts(ctx, after, limit)
}
