package zitadelprovision

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"task-processor/internal/authz"
	"task-processor/internal/zitadelprotojson"
)

// Used only by the explicit disposable, greenfield initializer. Serving never
// registers native roles or replaces a project grant.
// ResolveLocalAcceptanceOrganizations obtains the native IDs before a fixture
// constructs organization-bound role keys. Preferred IDs are lookup hints;
// keys are always derived from the actual IDs returned by ZITADEL.
func ResolveLocalAcceptanceOrganizations(ctx context.Context, cfg Config, names []string) ([]string, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if err := validateLocalIssuer(cfg.IssuerURL); err != nil {
		return nil, err
	}
	if !cfg.EnterpriseRoleSlots || len(names) != 2 || len(cfg.AcceptanceOrganizationIDs) != 2 {
		return nil, errors.New("invalid explicit enterprise fixture initialization")
	}
	if cfg.HTTPClient == nil {
		var err error
		cfg.HTTPClient, err = NewLoopbackOnlyHTTPClient(cfg.IssuerURL)
		if err != nil {
			return nil, err
		}
	}
	c := newClient(cfg)
	ids := make([]string, 2)
	for i, name := range names {
		var err error
		ids[i], err = c.ensureAcceptanceOrganization(ctx, cfg.AcceptanceOrganizationIDs[i], name)
		if err != nil {
			return nil, err
		}
	}
	if ids[0] == ids[1] {
		return nil, errors.New("enterprise fixture organizations are not distinct")
	}
	return ids, nil
}

func (c client) ensureEnterpriseRoleSlots(ctx context.Context, project, organization string) ([]string, error) {
	keys := []string{"listingkit_admin"}
	roles, err := c.listEnterpriseBootstrapRoles(ctx, project)
	if err != nil {
		return nil, err
	}
	for slot := 1; slot <= authz.EnterpriseRoleCapacity; slot++ {
		key := authz.EnterpriseRoleKey(organization, slot)
		keys = append(keys, key)
		if _, found := roles[key]; !found {
			err = c.createProjectRole(ctx, project, ProjectRole{Key: key, DisplayName: fmt.Sprintf("Enterprise role slot %02d", slot), Group: "Enterprise custom roles"})
			if err != nil && !isProviderConflict(err) {
				return nil, err
			}
		}
	}
	matched, err := waitForAcceptanceReadBack(ctx, func() (bool, error) {
		verified, err := c.listEnterpriseBootstrapRoles(ctx, project)
		if err != nil {
			return false, err
		}
		for _, key := range keys {
			if _, ok := verified[key]; !ok {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, errors.New("enterprise bootstrap role read-back incomplete")
	}
	return keys, nil
}
func (c client) listEnterpriseBootstrapRoles(ctx context.Context, project string) (map[string]ProjectRole, error) {
	result := map[string]ProjectRole{}
	for offset := 0; offset <= 1024; {
		var response struct {
			Details struct {
				Total zitadelprotojson.Uint64 `json:"totalResult"`
			} `json:"details"`
			Result []ProjectRole `json:"result"`
		}
		if err := c.doJSON(ctx, http.MethodPost, "/management/v1/projects/"+url.PathEscape(project)+"/roles/_search", map[string]any{"query": map[string]any{"offset": offset, "limit": 100, "asc": true}}, &response); err != nil {
			return nil, err
		}
		if response.Details.Total > 1024 || len(response.Result) > 100 || uint64(offset+len(response.Result)) > uint64(response.Details.Total) {
			return nil, errors.New("enterprise native role inventory exceeded bound")
		}
		for _, role := range response.Result {
			if role.Key == "" {
				return nil, errors.New("invalid native role inventory")
			}
			if _, duplicate := result[role.Key]; duplicate {
				return nil, errors.New("duplicate native role inventory")
			}
			result[role.Key] = role
		}
		offset += len(response.Result)
		if uint64(offset) >= uint64(response.Details.Total) {
			return result, nil
		}
		if len(response.Result) == 0 {
			return nil, errors.New("native role inventory made no progress")
		}
	}
	return nil, errors.New("native role inventory exceeded bound")
}
func (c client) ensureInitialEnterpriseGrant(ctx context.Context, project, organization string, keys []string) error {
	grants, err := c.listProjectGrants(ctx, project, organization)
	if err != nil {
		return err
	}
	if len(grants) > 1 {
		return errors.New("ambiguous enterprise project grant")
	}
	if len(grants) == 1 {
		if grants[0].State != "PROJECT_GRANT_STATE_ACTIVE" || !equalStringSets(grants[0].GrantedRoleKeys, keys) {
			return errors.New("existing enterprise project grant differs; bootstrap will not replace it")
		}
	} else if err = c.createProjectGrant(ctx, project, organization, keys); err != nil && !isProviderConflict(err) {
		return err
	}
	return c.verifyProjectGrant(ctx, project, organization, keys)
}
