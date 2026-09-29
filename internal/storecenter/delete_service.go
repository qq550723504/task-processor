package storecenter

import (
	"context"
	"time"
)

type NativeRecordDeleter interface {
	DeleteRecord(context.Context, DeleteStoreRequest, time.Time) (DeleteStoreResult, error)
}

func (s *Service) Delete(ctx context.Context, request DeleteStoreRequest) (DeleteStoreResult, error) {
	request, err := normalizeDeleteStoreRequest(request)
	if err != nil {
		return DeleteStoreResult{}, err
	}
	owner, ok := s.repository.(NativeRecordDeleter)
	if !ok {
		return DeleteStoreResult{}, ErrDependencyUnavailable
	}
	return owner.DeleteRecord(ctx, request, s.utcNow())
}
func exactSafeFields(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, field := range got {
		if seen[field] {
			return false
		}
		seen[field] = true
	}
	for _, field := range want {
		if !seen[field] {
			return false
		}
	}
	return true
}
