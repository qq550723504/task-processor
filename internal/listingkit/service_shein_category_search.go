package listingkit

import (
	"context"
)

func (s *service) SearchSheinCategories(ctx context.Context, taskID string, query string) (*SheinCategorySearchResult, error) {
	return s.sheinAdminOrDefault().SearchSheinCategories(ctx, taskID, query)
}
