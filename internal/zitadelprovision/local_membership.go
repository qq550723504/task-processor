package zitadelprovision

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"task-processor/internal/zitadelprotojson"
)

// ProvisionLocalAcceptanceMembershipWriter configures the dedicated local writer
// for the first (admin) acceptance organization only.
func ProvisionLocalAcceptanceMembershipWriter(ctx context.Context, cfg Config, writerID string) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := validateLocalIssuer(cfg.IssuerURL); err != nil {
		return err
	}
	writerID = strings.TrimSpace(writerID)
	if writerID == "" || cfg.OrgID == "" || len(cfg.AcceptanceOrganizationIDs) != 2 ||
		cfg.AcceptanceOrganizationIDs[0] == "" || cfg.AcceptanceOrganizationIDs[1] == "" ||
		cfg.AcceptanceOrganizationIDs[0] == cfg.AcceptanceOrganizationIDs[1] ||
		cfg.AcceptanceOrganizationIDs[0] == cfg.OrgID || cfg.AcceptanceOrganizationIDs[1] == cfg.OrgID {
		return errors.New("local membership writer requires its home and two distinct acceptance organizations")
	}
	if cfg.HTTPClient == nil {
		var err error
		cfg.HTTPClient, err = NewLoopbackOnlyHTTPClient(cfg.IssuerURL)
		if err != nil {
			return err
		}
	}
	c := newClient(cfg)
	var response struct {
		User struct {
			ID       string `json:"userId"`
			Username string `json:"username"`
			State    string `json:"state"`
			Machine  *struct {
				Name string `json:"name"`
			} `json:"machine"`
			Details struct {
				ResourceOwner string `json:"resourceOwner"`
			} `json:"details"`
		} `json:"user"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/users/"+url.PathEscape(writerID), nil, &response); err != nil {
		return fmt.Errorf("read local membership writer: %w", err)
	}
	u := response.User
	if u.ID != writerID || u.Username != "local-membership-write" || u.State != "USER_STATE_ACTIVE" || u.Machine == nil || u.Details.ResourceOwner != cfg.OrgID {
		return errors.New("local membership writer is not the expected active home service account")
	}
	c.orgID = cfg.AcceptanceOrganizationIDs[0]
	ready, err := c.readAcceptanceMembershipWriter(ctx, writerID, cfg.OrgID)
	if err != nil || ready {
		return err
	}
	// Add the fixed native role only after an exact empty read. Never replace
	// unexpected roles or grant instance privileges to make this fixture pass.
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/orgs/me/members", map[string]any{
		"userId": writerID, "roles": []string{"ORG_USER_MANAGER"},
	}, &struct{}{}); err != nil && !isProviderConflict(err) {
		return fmt.Errorf("grant local membership writer: %w", err)
	}
	ready, err = waitForAcceptanceReadBack(ctx, func() (bool, error) { return c.readAcceptanceMembershipWriter(ctx, writerID, cfg.OrgID) })
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("local membership writer role read-back is not ready")
	}
	return nil
}

func (c client) readAcceptanceMembershipWriter(ctx context.Context, writerID, homeID string) (bool, error) {
	var response struct {
		Details *struct {
			// ZITADEL v4.17.1 omits the protobuf zero count on an empty page.
			TotalResult zitadelprotojson.Uint64 `json:"totalResult"`
		} `json:"details"`
		Result []struct {
			UserID            string   `json:"userId"`
			UserResourceOwner string   `json:"userResourceOwner"`
			Roles             []string `json:"roles"`
			Details           struct {
				ResourceOwner string `json:"resourceOwner"`
			} `json:"details"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/management/v1/orgs/me/members/_search", map[string]any{
		"query": map[string]any{"limit": 2}, "queries": []any{map[string]any{"userIdQuery": map[string]string{"userId": writerID}}},
	}, &response); err != nil {
		return false, fmt.Errorf("read local membership writer role: %w", err)
	}
	if response.Details == nil || uint64(response.Details.TotalResult) != uint64(len(response.Result)) || len(response.Result) > 1 {
		return false, errors.New("local membership writer role read-back is ambiguous")
	}
	if len(response.Result) == 0 {
		return false, nil
	}
	m := response.Result[0]
	if m.UserID != writerID || m.UserResourceOwner != homeID || m.Details.ResourceOwner != c.orgID || len(m.Roles) != 1 || m.Roles[0] != "ORG_USER_MANAGER" {
		return false, errors.New("local membership writer role does not match the exact organization and role")
	}
	return true, nil
}
