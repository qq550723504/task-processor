package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/authz"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type unifiedSummaryFixture struct {
	fail bool
	org  string
}

func (r *unifiedSummaryFixture) ReadServiceSummary(_ context.Context, org string, _ time.Time) (storecenter.ServiceSummary, error) {
	r.org = org
	if r.fail {
		return storecenter.ServiceSummary{}, errors.New("synthetic read failure")
	}
	return storecenter.ServiceSummary{Records: 3, Active: 2}, nil
}

func TestUnifiedOverviewUsesIndependentAvailabilityAndTrustedScope(t *testing.T) {
	resources := &resourceBalanceFixture{}
	stores := &unifiedSummaryFixture{fail: true}
	module := unifiedCommercialModule{resources: resources, stores: stores}
	routes := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(routes))
	grants := &auditHTTPGrants{role: "listingkit_operator"}
	server := buildIsolatedApplicationHTTPServer(routes.Routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authz.DefaultListingKitAuthorizer()}, 15*time.Second)
	get := func(path, org, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer fixture")
		request.Header.Set("X-Requested-Organization-ID", org)
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, request)
		return response
	}
	response := get("/api/v1/workbench/commercial/overview", "B", "")
	require.Equal(t, 200, response.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Equal(t, "unified-base-prepaid-v1", payload["schema_version"])
	require.Equal(t, "B", payload["organization_id"])
	require.Equal(t, false, payload["base_plan"].(map[string]any)["subscription_required"])
	require.Equal(t, "available", payload["resources"].(map[string]any)["state"])
	require.Equal(t, "unavailable", payload["store_services"].(map[string]any)["state"])
	require.Equal(t, "B", stores.org)
	require.Contains(t, response.Header().Get("Cache-Control"), "no-store")
	resources.failure = true
	stores.fail = false
	response = get("/api/v1/workbench/commercial/overview", "B", "")
	require.Equal(t, 200, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Equal(t, "unavailable", payload["resources"].(map[string]any)["state"])
	require.Equal(t, "available", payload["store_services"].(map[string]any)["state"])
	for _, test := range []struct {
		path, org, body string
		code            int
	}{{"/api/v1/workbench/commercial/overview?organization_id=A", "B", "", 400}, {"/api/v1/workbench/commercial/overview", "A", "", 403}, {"/api/v1/workbench/commercial/overview", "B", "{}", 400}} {
		before := resources.calls
		response = get(test.path, test.org, test.body)
		require.Equal(t, test.code, response.Code)
		require.Equal(t, before, resources.calls)
	}
}

type resourceEventFixture struct {
	query orgresource.EventQuery
	calls int
}

func (r *resourceEventFixture) ListEvents(_ context.Context, q orgresource.EventQuery) (orgresource.EventPage, error) {
	r.calls++
	r.query = q
	if q.Cursor == "invalid" {
		return orgresource.EventPage{}, orgresource.ErrInvalidInput
	}
	return orgresource.EventPage{OrganizationID: q.OrganizationID, Items: []orgresource.EventView{}}, nil
}
func TestResourceEventsUseBoundedFiltersAndLiveOrganization(t *testing.T) {
	reader := &resourceEventFixture{}
	grants := &auditHTTPGrants{role: "listingkit_operator"}
	module := commercialResourcesModule{reader: &resourceBalanceFixture{}, events: reader}
	server := buildIsolatedApplicationHTTPServer(module.routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authz.DefaultListingKitAuthorizer()}, 15*time.Second)
	get := func(suffix, org, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("GET", commercialResourceEventsPath+suffix, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer fixture")
		request.Header.Set("X-Requested-Organization-ID", org)
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, request)
		return response
	}
	response := get("?resource_type=ai_point&limit=2&from=2026-09-01T00:00:00Z&until=2026-10-01T00:00:00Z", "B", "")
	require.Equal(t, 200, response.Code)
	require.Equal(t, "B", reader.query.OrganizationID)
	require.Equal(t, 2, reader.query.Limit)
	require.Equal(t, orgresource.ResourceAIPoint, reader.query.ResourceType)
	for _, query := range []string{"?organization_id=A", "?limit=51", "?limit=2&limit=3", "?resource_type=token", "?from=bad", "?from=2026-10-01T00:00:00Z&until=2026-09-01T00:00:00Z", "?"} {
		before := reader.calls
		require.Equal(t, 400, get(query, "B", "").Code)
		require.Equal(t, before, reader.calls)
	}
	require.Equal(t, 400, get("?cursor=invalid", "B", "").Code)
	before := reader.calls
	require.Equal(t, 403, get("", "A", "").Code)
	require.Equal(t, 400, get("", "B", "{}").Code)
	grants.revoked = true
	require.Equal(t, 403, get("", "B", "").Code)
	require.Equal(t, before, reader.calls)
}
