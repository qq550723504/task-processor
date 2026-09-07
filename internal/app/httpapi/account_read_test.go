package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/workbenchcontext"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type accountFixture struct {
	server       *httptest.Server
	provider     *httptest.Server
	revoked      atomic.Bool
	unavailable  atomic.Bool
	clock        atomic.Int64
	grantReads   atomic.Int32
	profileReads atomic.Int32
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	f := &accountFixture{}
	f.clock.Store(time.Now().Unix())
	f.provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"introspection_endpoint": f.provider.URL + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			require.Equal(t, "POST", r.Method)
			require.NoError(t, r.ParseForm())
			token = r.Form.Get("token")
			if token == "auth-down" {
				w.WriteHeader(503)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": token != "expired", "sub": token, "exp": time.Now().Add(time.Hour).Unix(), "urn:zitadel:iam:user:resourceowner:id": "A"})
		case "/oidc/v1/userinfo":
			f.profileReads.Add(1)
			if token == "down" {
				w.WriteHeader(503)
				_, _ = w.Write([]byte(`{"secret":"provider-private"}`))
				return
			}
			if token == "slow" {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(20 * time.Second):
					return
				}
			}
			sub := token
			if token == "mismatch" {
				sub = "someone-else"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": sub, "name": "Fixture " + token, "email": "U-OPAQUE@PHONE.INVALID", "email_verified": true})
		case "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			f.grantReads.Add(1)
			require.Equal(t, "POST", r.Method)
			if f.unavailable.Load() || token == "grant-down" {
				w.WriteHeader(503)
				return
			}
			var request struct {
				Filters []struct {
					InUserIDs struct {
						IDs []string `json:"ids"`
					} `json:"inUserIds"`
					ProjectID struct {
						ID string `json:"id"`
					} `json:"projectId"`
				} `json:"filters"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, []string{token}, request.Filters[0].InUserIDs.IDs)
			require.Equal(t, "project", request.Filters[1].ProjectID.ID)
			rows := []any{}
			if !f.revoked.Load() && token != "no-org" {
				for _, org := range []string{"B", "C"} {
					rows = append(rows, map[string]any{"id": "grant-" + org, "project": map[string]string{"id": "project"}, "organization": map[string]string{"id": org, "name": "Enterprise " + org}, "user": map[string]string{"id": token}, "state": "STATE_ACTIVE", "roles": []any{map[string]string{"key": "listingkit_viewer"}}})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"pagination": map[string]string{"totalResult": fmt.Sprint(len(rows))}, "authorizations": rows})
		default:
			t.Errorf("unexpected external path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.provider.Close)
	factories := defaultWorkbenchContextFactories()
	factories.newGrantCache = func() *workbenchcontext.GrantCache {
		return workbenchcontext.NewGrantCache(func() time.Time { return time.Unix(f.clock.Load(), 0) })
	}
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: f.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: f.provider.URL}}}
	built, err := buildWorkbenchContextModule(cfg, logger, factories)
	require.NoError(t, err)
	reg := kernelmodule.NewRegistry()
	require.NoError(t, built.module.Register(reg))
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, reg.Routes(), *built.authDependencies)
	f.server = httptest.NewServer(server.Handler)
	t.Cleanup(f.server.Close)
	return f
}

func (f *accountFixture) request(t *testing.T, method, path, token, org, body string) (int, map[string]any) {
	t.Helper()
	r, err := http.NewRequest(method, f.server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-User-ID", "forged")
	r.Header.Set("X-Tenant-ID", "forged")
	if org != "" {
		r.Header.Set("X-Requested-Organization-ID", org)
	}
	response, err := f.server.Client().Do(r)
	require.NoError(t, err)
	defer response.Body.Close()
	var result map[string]any
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	if strings.Contains(path, "/account/") {
		require.Contains(t, response.Header.Get("Cache-Control"), "no-store")
	}
	return response.StatusCode, result
}

func TestAccountActualReadersAndScope(t *testing.T) {
	f := newAccountFixture(t)
	status, profile := f.request(t, "GET", "/api/v1/account/profile", "u1", "revoked-org", "")
	require.Equal(t, 200, status)
	require.Equal(t, "u1", profile["userId"])
	require.Equal(t, "A", profile["homeOrganizationId"])
	require.Nil(t, profile["email"])
	require.Nil(t, profile["emailVerified"])
	require.Zero(t, f.grantReads.Load())
	status, org := f.request(t, "GET", "/api/v1/account/organization", "u1", "B", "")
	require.Equal(t, 200, status)
	require.Equal(t, "A", org["homeOrganizationId"])
	require.Equal(t, "B", org["effectiveOrganizationId"])
	require.Equal(t, []any{"listingkit_viewer"}, org["roles"])
	status, switched := f.request(t, "PUT", "/api/v1/workbench/context/effective-organization", "u1", "C", `{"organizationId":"C"}`)
	require.Equal(t, 200, status)
	require.Equal(t, "C", switched["effectiveOrganizationId"])
	status, org = f.request(t, "GET", "/api/v1/account/organization", "u1", "C", "")
	require.Equal(t, 200, status)
	require.Equal(t, "C", org["effectiveOrganizationId"])
	for _, tc := range []struct {
		path, token, org, body string
		status                 int
		code                   string
	}{
		{"organization", "u1", "", "", 409, "ORGANIZATION_SELECTION_REQUIRED"},
		{"organization", "u1", "A", "", 403, "ORGANIZATION_ACCESS_DENIED"},
		{"organization", "no-org", "B", "", 403, "ORGANIZATION_ACCESS_REVOKED"},
		{"organization", "grant-down", "B", "", 503, "DEPENDENCY_UNAVAILABLE"},
		{"profile", "no-org", "", "", 200, ""},
		{"profile", "grant-down", "", "", 200, ""},
		{"profile", "mismatch", "", "", 502, "INVALID_UPSTREAM_RESPONSE"},
		{"profile", "down", "", "", 503, "DEPENDENCY_UNAVAILABLE"},
		{"profile", "expired", "", "", 401, "AUTHENTICATION_REQUIRED"},
		{"profile", "auth-down", "", "", 503, "DEPENDENCY_UNAVAILABLE"},
		{"profile?user=u2", "u1", "", "", 400, "INVALID_REQUEST"},
		{"profile", "u1", "", "{}", 400, "INVALID_REQUEST"},
	} {
		t.Run(tc.path+tc.token+tc.org+tc.body, func(t *testing.T) {
			status, value := f.request(t, "GET", "/api/v1/account/"+tc.path, tc.token, tc.org, tc.body)
			require.Equal(t, tc.status, status, value)
			if tc.code != "" {
				require.Equal(t, tc.code, value["code"])
				require.NotContains(t, fmt.Sprint(value), "provider-private")
			}
		})
	}
}

func TestAccountReadRevocationCacheAndSwitch(t *testing.T) {
	f := newAccountFixture(t)
	status, _ := f.request(t, "GET", "/api/v1/account/organization", "u1", "B", "")
	require.Equal(t, 200, status)
	f.revoked.Store(true)
	status, _ = f.request(t, "GET", "/api/v1/account/organization", "u1", "B", "")
	require.Equal(t, 200, status, "approved cached read window")
	f.clock.Add(61)
	status, value := f.request(t, "GET", "/api/v1/account/organization", "u1", "B", "")
	require.Equal(t, 403, status)
	require.Equal(t, "ORGANIZATION_ACCESS_REVOKED", value["code"])
	f.revoked.Store(false)
	status, _ = f.request(t, "PUT", "/api/v1/workbench/context/effective-organization", "u1", "B", `{"organizationId":"B"}`)
	require.Equal(t, 200, status)
	f.revoked.Store(true)
	status, _ = f.request(t, "PUT", "/api/v1/workbench/context/effective-organization", "u1", "B", `{"organizationId":"B"}`)
	require.Equal(t, 403, status)
	status, _ = f.request(t, "GET", "/api/v1/account/organization", "u1", "B", "")
	require.Equal(t, 403, status, "switch invalidates cached reads")
}

func TestAccountAuthenticationErrorsHaveBoundedRequestID(t *testing.T) {
	f := newAccountFixture(t)
	for _, token := range []string{"", "expired", "no-org"} {
		request, err := http.NewRequest("GET", f.server.URL+"/api/v1/account/organization", nil)
		require.NoError(t, err)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		request.Header.Set("X-Requested-Organization-ID", "B")
		request.Header.Set("X-Request-ID", strings.Repeat("x", 20000))
		response, err := f.server.Client().Do(request)
		require.NoError(t, err)
		data, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.LessOrEqual(t, len(data), 16*1024)
	}
}

// This test always exercises real internal HTTP/reader assembly. When a task
// directory is provided it also serves the separate Next/client acceptance run.
func TestAccountBrowserFixture(t *testing.T) {
	f := newAccountFixture(t)
	status, _ := f.request(t, "GET", "/api/v1/account/profile", "u1", "", "")
	require.Equal(t, 200, status)
	dir := os.Getenv("ISSUE346_FIXTURE_DIR")
	if dir == "" {
		return
	}
	require.True(t, filepath.IsAbs(dir))
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		switch r.URL.Path {
		case "/revoke":
			f.revoked.Store(true)
		case "/restore":
			f.revoked.Store(false)
		case "/expire":
			f.clock.Add(61)
		case "/unavailable":
			f.unavailable.Store(true)
		default:
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(204)
	}))
	defer control.Close()
	data, err := json.Marshal(map[string]any{"goOrigin": f.server.URL, "providerOrigin": f.provider.URL, "controlOrigin": control.URL, "tokens": []string{"u1", "u2", "no-org", "grant-down", "down", "mismatch", "expired", "slow"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.json"), data, 0600))
	deadline := time.NewTimer(29 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("fixture expired")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(dir, "stop-go")); err == nil {
				return
			}
		}
	}
}

func TestAccountCurrentIdentitySkipsLegacyAndOrganization(t *testing.T) {
	identity := mountedVerifiedIdentity()
	router := gin.New()
	mountRoutesWithAuthDependencies(router, []httproute.Descriptor{{
		Method: "GET", Path: "/api/v1/account/profile", AuthPolicy: httproute.AuthPolicy("current_identity"),
		OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone,
		Handler: func(c *gin.Context) {
			got, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
			require.True(t, ok)
			require.Equal(t, identity.UserID, got.UserID)
			require.Equal(t, identity.HomeOrganizationID, got.HomeOrganizationID)
			require.Empty(t, got.TenantID)
			require.Empty(t, got.Roles)
			c.Status(http.StatusNoContent)
		},
	}}, routeAuthDependencies{
		workbenchVerifier:  mountedVerifierStub{identity: identity},
		identityMiddleware: func(c *gin.Context) { t.Fatal("legacy authentication used") },
	})
	request := httptest.NewRequest("GET", "/api/v1/account/profile", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("X-Requested-Organization-ID", "revoked")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest("GET", "/api/v1/account/profile", nil))
	require.Equal(t, http.StatusUnauthorized, missing.Code)
}
