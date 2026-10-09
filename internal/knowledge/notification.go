package knowledge

import "context"

type NoticeSourceReader interface {
	ListNoticeSources(context.Context, string, string, int) ([]Source, string, error)
}

// The serving caller must bind the same live Knowledge read capability as
// the existing HTTP entry. This service still checks its original scope.
func (s *Service) NotificationFacts(ctx context.Context, scope Scope, after string, limit int) ([]Source, string, error) {
	if s == nil {
		return nil, "", ErrUnavailable
	}
	if !validScope(scope) {
		return nil, "", ErrInvalid
	}
	r, ok := s.repo.(NoticeSourceReader)
	if !ok {
		return nil, "", ErrUnavailable
	}
	return r.ListNoticeSources(ctx, scope.OrganizationID, after, limit)
}
