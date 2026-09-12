package membership

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"task-processor/internal/authidentity"
	domain "task-processor/internal/organization/membership"
)

// The native permission API retains project/grant context as a colon suffix.
// Only a bare permission in the explicitly selected organization proves this
// dedicated org credential can read the complete directory. Never strip suffixes.
func (c *Client) requirePermission(ctx context.Context, organization, permission string) error {
	if !authidentity.IsBoundedIdentifier(organization) {
		return domain.ErrInvalidRequest
	}
	raw, err := c.requestJSON(ctx, http.MethodPost, "/auth/v1/permissions/zitadel/me/_search", nil, false, organization)
	if err != nil {
		return domain.ErrUnavailable
	}
	var result struct {
		Permissions []string `json:"result"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Permissions) > 1024 || !slices.Contains(result.Permissions, permission) {
		return domain.ErrUnavailable
	}
	return nil
}
