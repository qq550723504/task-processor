package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

type workbenchBodyBoundaryReader struct {
	payload          []byte
	offset           int
	readsPastPayload int
}

func (reader *workbenchBodyBoundaryReader) Read(buffer []byte) (int, error) {
	if reader.offset >= len(reader.payload) {
		reader.readsPastPayload++
		return 0, io.EOF
	}
	read := copy(buffer, reader.payload[reader.offset:])
	reader.offset += read
	return read, nil
}

func TestWorkbenchContextModuleRegistersBothPoliciesExactlyOnce(t *testing.T) {
	module := NewModule(NewHandler())
	require.False(t, module.Enabled(&config.Config{}))
	require.True(t, module.Enabled(&config.Config{Workbench: config.WorkbenchConfig{Enabled: true}}))
	registry := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(registry))

	routes := registry.Routes()
	require.Len(t, routes, 2)
	require.Equal(t, http.MethodGet, routes[0].Method)
	require.Equal(t, "/api/v1/workbench/context", routes[0].Path)
	require.Equal(t, httproute.AuthPolicyVerifiedIdentity, routes[0].AuthPolicy)
	require.Equal(t, httproute.OrganizationAccessPolicyContextRead, routes[0].OrganizationAccessPolicy)
	require.Nil(t, routes[0].OrganizationTargetResolver)
	require.Equal(t, http.MethodPut, routes[1].Method)
	require.Equal(t, "/api/v1/workbench/context/effective-organization", routes[1].Path)
	require.Equal(t, httproute.AuthPolicyVerifiedIdentity, routes[1].AuthPolicy)
	require.Equal(t, httproute.OrganizationAccessPolicyLiveSwitch, routes[1].OrganizationAccessPolicy)
	require.NotNil(t, routes[1].OrganizationTargetResolver)
}

func TestWorkbenchContextSwitchTargetStrictlyRejectsUntrustedCandidatesAndRestoresBody(t *testing.T) {
	testCases := []struct {
		name   string
		body   string
		header string
	}{
		{name: "unknown field", body: `{"organizationId":"org-a","extra":true}`},
		{name: "duplicate field", body: `{"organizationId":"org-a","organizationId":"org-b"}`},
		{name: "wrong field casing", body: `{"OrganizationId":"org-a"}`},
		{name: "blank id", body: `{"organizationId":"   "}`},
		{name: "trailing JSON", body: `{"organizationId":"org-a"}{"organizationId":"org-b"}`},
		{name: "missing body", body: ``},
		{name: "header mismatch", body: `{"organizationId":"org-a"}`, header: "org-b"},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", strings.NewReader(tt.body))
			request.Header.Set("X-Requested-Organization-ID", tt.header)

			_, err := ResolveSwitchOrganizationTarget(request)

			require.Error(t, err)
			restored, readErr := io.ReadAll(request.Body)
			require.NoError(t, readErr)
			require.Equal(t, tt.body, string(restored))
		})
	}
}

func TestWorkbenchContextSwitchTargetAcceptsExactBodyAndMatchingHeader(t *testing.T) {
	const body = `{"organizationId":"org-a"}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", strings.NewReader(body))
	request.Header.Set("X-Requested-Organization-ID", "org-a")

	target, err := ResolveSwitchOrganizationTarget(request)

	require.NoError(t, err)
	require.Equal(t, "org-a", target)
	restored, readErr := io.ReadAll(request.Body)
	require.NoError(t, readErr)
	require.Equal(t, body, string(restored))
}

func TestWorkbenchContextSwitchTargetRejects4097ByteBodyWithoutReadingPastLimit(t *testing.T) {
	reader := &workbenchBodyBoundaryReader{payload: []byte(strings.Repeat("x", 4097))}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", reader)

	_, err := ResolveSwitchOrganizationTarget(request)

	require.Error(t, err)
	require.Zero(t, reader.readsPastPayload)
}

func TestWorkbenchContextSwitchTargetAccepts4096ByteBodyAndRestoresIt(t *testing.T) {
	body := `{"organizationId":"` + strings.Repeat("a", 4075) + `"}`
	require.Len(t, body, 4096)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/workbench/context/effective-organization", strings.NewReader(body))

	target, err := ResolveSwitchOrganizationTarget(request)

	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 4075), target)
	restored, readErr := io.ReadAll(request.Body)
	require.NoError(t, readErr)
	require.Equal(t, body, string(restored))
}

func TestWorkbenchContextGetProjectsOnlyPublicIdentityContract(t *testing.T) {
	response := serveHandler(t, http.MethodGet, "/api/v1/workbench/context", "", authidentity.AuthenticatedIdentity{
		UserID:                  "user-1",
		HomeOrganizationID:      "org-a",
		EffectiveOrganizationID: "org-b",
		TenantID:                "org-b",
		Roles:                   []string{"listingkit_editor"},
		TokenExpiresAt:          time.Unix(2_000_000_000, 0),
		OrganizationGrants: []authidentity.OrganizationGrant{
			{OrganizationID: "org-a", OrganizationName: "Organization A", ProjectID: "project-secret", Roles: []string{"listingkit_admin"}},
		},
	}, NewHandler().GetContext)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"user":{"id":"user-1"},
		"homeOrganizationId":"org-a",
		"effectiveOrganizationId":"org-b",
		"selectionRequired":false,
		"organizations":[{"id":"org-a","name":"Organization A","roles":["listingkit_admin"]}]
	}`, response.Body.String())
	for _, forbidden := range []string{"project-secret", "tokenExpiresAt", "authorizationId", "source", "bearer"} {
		require.NotContains(t, response.Body.String(), forbidden)
	}
}

func TestWorkbenchContextGetReturnsSelectionRequiredWithNullEffectiveOrganization(t *testing.T) {
	response := serveHandler(t, http.MethodGet, "/api/v1/workbench/context", "", authidentity.AuthenticatedIdentity{
		UserID:             "user-1",
		HomeOrganizationID: "org-home-without-grant",
		OrganizationGrants: []authidentity.OrganizationGrant{
			{OrganizationID: "org-a", OrganizationName: "Organization A", Roles: []string{"role-a"}},
			{OrganizationID: "org-b", OrganizationName: "Organization B", Roles: []string{"role-b"}},
		},
	}, NewHandler().GetContext)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"user":{"id":"user-1"},
		"homeOrganizationId":"org-home-without-grant",
		"effectiveOrganizationId":null,
		"selectionRequired":true,
		"organizations":[
			{"id":"org-a","name":"Organization A","roles":["role-a"]},
			{"id":"org-b","name":"Organization B","roles":["role-b"]}
		]
	}`, response.Body.String())
}

func TestWorkbenchContextGetReturnsExplicitNoAccessWithOrganizationsArray(t *testing.T) {
	response := serveHandler(t, http.MethodGet, "/api/v1/workbench/context", "", authidentity.AuthenticatedIdentity{
		UserID:             "user-1",
		HomeOrganizationID: "org-a",
	}, NewHandler().GetContext)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"user":{"id":"user-1"},
		"homeOrganizationId":"org-a",
		"effectiveOrganizationId":null,
		"selectionRequired":false,
		"organizations":[]
	}`, response.Body.String())
}

func TestWorkbenchContextPutReturnsSamePublicContractAfterLiveSwitch(t *testing.T) {
	response := serveHandler(t, http.MethodPut, "/api/v1/workbench/context/effective-organization", `{"organizationId":"org-b"}`, authidentity.AuthenticatedIdentity{
		UserID:                  "user-1",
		HomeOrganizationID:      "org-a",
		EffectiveOrganizationID: "org-b",
		OrganizationGrants: []authidentity.OrganizationGrant{
			{OrganizationID: "org-b", OrganizationName: "Organization B", Roles: []string{"listingkit_admin"}},
		},
	}, NewHandler().SwitchEffectiveOrganization)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"user":{"id":"user-1"},
		"homeOrganizationId":"org-a",
		"effectiveOrganizationId":"org-b",
		"selectionRequired":false,
		"organizations":[{"id":"org-b","name":"Organization B","roles":["listingkit_admin"]}]
	}`, response.Body.String())
}

func serveHandler(t *testing.T, method string, path string, body string, identity authidentity.AuthenticatedIdentity, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, path, func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), identity))
		c.Next()
	}, handler)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}
