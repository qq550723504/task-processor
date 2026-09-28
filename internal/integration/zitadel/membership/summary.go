package membership

import (
	"context"
	domain "task-processor/internal/organization/membership"
)

func (c *Client) Counts(ctx context.Context, org string) (domain.Counts, error) {
	result := domain.Counts{}
	for _, query := range []struct {
		target      *int
		filter      any
		state, role string
	}{
		{&result.Total, nil, "", ""},
		{&result.Active, map[string]any{"state": map[string]string{"state": "STATE_ACTIVE"}}, "active", ""},
		{&result.Inactive, map[string]any{"state": map[string]string{"state": "STATE_INACTIVE"}}, "inactive", ""},
		{&result.Administrators, map[string]any{"roleKey": map[string]string{"key": "listingkit_admin"}}, "", "listingkit_admin"},
	} {
		var extra []any
		if query.filter != nil {
			extra = []any{query.filter}
		}
		page, err := c.list(ctx, org, domain.PageRequest{Limit: 1}, "", extra...)
		if err != nil {
			return domain.Counts{}, err
		}
		for _, member := range page.Items {
			if query.state != "" && member.State != query.state {
				return domain.Counts{}, domain.ErrInvalidResponse
			}
			if query.role != "" {
				found := false
				for _, role := range member.Roles {
					found = found || role == query.role
				}
				if !found {
					return domain.Counts{}, domain.ErrInvalidResponse
				}
			}
		}
		*query.target = page.Total
	}
	return result, nil
}
