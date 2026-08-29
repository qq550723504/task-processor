package zitadelprovision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-processor/internal/authidentity"
)

func TestProvisionLocalApplicationsCreatesAPIAndOIDCApps(t *testing.T) {
	var apiBody map[string]any
	var oidcBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles/_search":
			writeJSON(t, w, map[string]any{"result": defaultRoleResponses()})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/_search":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode app search body: %v", err)
			}
			if len(body) != 0 {
				t.Fatalf("app search body = %#v, want empty query", body)
			}
			writeJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/api":
			if err := json.NewDecoder(r.Body).Decode(&apiBody); err != nil {
				t.Fatalf("decode API app body: %v", err)
			}
			writeJSON(t, w, map[string]any{
				"appId": "api-app-1", "clientId": "api-client-1", "clientSecret": "api-secret-1",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/apps/oidc":
			if err := json.NewDecoder(r.Body).Decode(&oidcBody); err != nil {
				t.Fatalf("decode OIDC app body: %v", err)
			}
			writeJSON(t, w, map[string]any{
				"appId": "oidc-app-1", "clientId": "oidc-client-1", "clientSecret": "oidc-secret-1",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := ProvisionLocalApplications(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1",
	}, LocalApplicationConfig{
		APIName:                "ListingKit Local API",
		OIDCName:               "ListingKit Local OIDC",
		RedirectURIs:           []string{"http://localhost:3000/api/zitadel-auth/callback"},
		PostLogoutRedirectURIs: []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("ProvisionLocalApplications returned error: %v", err)
	}

	if apiBody["name"] != "ListingKit Local API" || apiBody["authMethodType"] != "API_AUTH_METHOD_TYPE_BASIC" {
		t.Fatalf("API app body = %#v", apiBody)
	}
	assertStringSlice(t, oidcBody["redirectUris"], []string{"http://localhost:3000/api/zitadel-auth/callback"})
	assertStringSlice(t, oidcBody["postLogoutRedirectUris"], []string{"http://localhost:3000"})
	if oidcBody["name"] != "ListingKit Local OIDC" ||
		oidcBody["appType"] != "OIDC_APP_TYPE_USER_AGENT" ||
		oidcBody["authMethodType"] != "OIDC_AUTH_METHOD_TYPE_NONE" ||
		oidcBody["accessTokenType"] != "OIDC_TOKEN_TYPE_BEARER" ||
		oidcBody["accessTokenRoleAssertion"] != true {
		t.Fatalf("OIDC app body = %#v", oidcBody)
	}
	assertStringSlice(t, oidcBody["responseTypes"], []string{"OIDC_RESPONSE_TYPE_CODE"})
	assertStringSlice(t, oidcBody["grantTypes"], []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"})

	if result.ProjectID != "project-1" || result.APIAppID != "api-app-1" || result.APIClientID != "api-client-1" ||
		result.APIClientSecret != "api-secret-1" || result.OIDCAppID != "oidc-app-1" ||
		result.OIDCClientID != "oidc-client-1" || result.OIDCClientSecret != "oidc-secret-1" {
		t.Fatalf("result = %#v", result)
	}
	formatted := fmt.Sprintf("%v", result)
	for _, secret := range []string{"token", "api-secret-1", "oidc-secret-1"} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("formatted result leaked %q: %s", secret, formatted)
		}
	}
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:role:listingkit_operator") {
		t.Fatalf("recommended scopes = %#v", result.RecommendedScopes)
	}
}

func TestProvisionLocalApplicationsReusesAppsByStableName(t *testing.T) {
	createCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/management/v1/projects/project-1/roles/_search":
			writeJSON(t, w, map[string]any{"result": defaultRoleResponses()})
		case "/management/v1/projects/project-1/apps/_search":
			writeJSON(t, w, map[string]any{"result": []map[string]any{
				{"id": "api-app-1", "name": "ListingKit Local API", "apiConfig": map[string]any{"clientId": "api-client-1", "authMethodType": "API_AUTH_METHOD_TYPE_BASIC"}},
				{"id": "oidc-app-1", "name": "ListingKit Local OIDC", "oidcConfig": map[string]any{"clientId": "oidc-client-1"}},
			}})
		case "/management/v1/projects/project-1/apps/oidc-app-1":
			writeJSON(t, w, map[string]any{"app": map[string]any{
				"id": "oidc-app-1", "name": "ListingKit Local OIDC", "oidcConfig": map[string]any{
					"clientId": "oidc-client-1", "redirectUris": []string{"http://localhost:3000/api/zitadel-auth/callback"},
					"responseTypes": []string{"OIDC_RESPONSE_TYPE_CODE"}, "grantTypes": []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"},
					"appType": "OIDC_APP_TYPE_USER_AGENT", "authMethodType": "OIDC_AUTH_METHOD_TYPE_NONE",
					"postLogoutRedirectUris": []string{"http://localhost:3000"}, "accessTokenType": "OIDC_TOKEN_TYPE_BEARER", "accessTokenRoleAssertion": true,
				},
			}})
		case "/management/v1/projects/project-1/apps/api", "/management/v1/projects/project-1/apps/oidc":
			createCalls++
			writeJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := ProvisionLocalApplications(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1",
	}, LocalApplicationConfig{
		APIName: "ListingKit Local API", OIDCName: "ListingKit Local OIDC",
		RedirectURIs:           []string{"http://localhost:3000/api/zitadel-auth/callback"},
		PostLogoutRedirectURIs: []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("ProvisionLocalApplications returned error: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", createCalls)
	}
	if result.APIAppID != "api-app-1" || result.APIClientID != "api-client-1" || result.APIClientSecret != "" ||
		result.OIDCAppID != "oidc-app-1" || result.OIDCClientID != "oidc-client-1" || result.OIDCClientSecret != "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestProvisionLocalApplicationsRejectsNonLocalIssuerOrRedirect(t *testing.T) {
	base := Config{IssuerURL: "https://zitadel.example.com", ManagementToken: "token", ProjectID: "project-1"}
	appCfg := LocalApplicationConfig{
		APIName: "ListingKit Local API", OIDCName: "ListingKit Local OIDC",
		RedirectURIs: []string{"http://localhost:3000/api/zitadel-auth/callback"}, PostLogoutRedirectURIs: []string{"http://localhost:3000"},
	}
	if _, err := ProvisionLocalApplications(context.Background(), base, appCfg); err == nil {
		t.Fatal("remote issuer was accepted")
	}
	base.IssuerURL = "http://127.0.0.1:8080"
	appCfg.RedirectURIs = []string{"https://example.com/callback"}
	if _, err := ProvisionLocalApplications(context.Background(), base, appCfg); err == nil {
		t.Fatal("non-local redirect was accepted")
	}
}

func TestGrantLocalOperatorUsesVerifiedIdentity(t *testing.T) {
	var grantBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/management/v1/users/grants/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
				t.Fatalf("Connect-Protocol-Version = %q, want 1", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&grantBody); err != nil {
				t.Fatalf("decode grant body: %v", err)
			}
			writeJSON(t, w, map[string]any{"id": "authorization-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := GrantLocalOperator(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1",
	}, "", authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("GrantLocalOperator returned error: %v", err)
	}
	if grantBody["userId"] != "user-1" || grantBody["organizationId"] != "org-1" || grantBody["projectId"] != "project-1" {
		t.Fatalf("grant body = %#v", grantBody)
	}
	assertStringSlice(t, grantBody["roleKeys"], []string{"listingkit_operator"})
}

func TestGrantLocalOperatorIsIdempotentAndAddsAdminOnlyWhenRequested(t *testing.T) {
	tests := []struct {
		name            string
		additionalRole  string
		existingRoles   []string
		wantUpdateRoles []string
	}{
		{name: "operator already exists", existingRoles: []string{"listingkit_operator"}},
		{name: "admin is added without revoking roles", additionalRole: "listingkit_admin", existingRoles: []string{"listingkit_viewer", "listingkit_operator"}, wantUpdateRoles: []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createCalls := 0
			updateCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requireAuth(t, r)
				switch r.URL.Path {
				case "/management/v1/users/grants/_search":
					writeJSON(t, w, map[string]any{"result": []map[string]any{{
						"id": "authorization-1", "userId": "user-1", "orgId": "org-1", "projectId": "project-1", "roleKeys": tt.existingRoles,
					}}})
				case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
					createCalls++
					writeJSON(t, w, map[string]any{"id": "authorization-2"})
				case "/zitadel.authorization.v2.AuthorizationService/UpdateAuthorization":
					updateCalls++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["id"] != "authorization-1" {
						t.Fatalf("update id = %#v", body["id"])
					}
					assertStringSlice(t, body["roleKeys"], tt.wantUpdateRoles)
					writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			err := GrantLocalOperator(context.Background(), Config{
				IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1",
			}, tt.additionalRole, authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1"})
			if err != nil {
				t.Fatalf("GrantLocalOperator returned error: %v", err)
			}
			if createCalls != 0 {
				t.Fatalf("create calls = %d, want 0", createCalls)
			}
			wantUpdates := 0
			if len(tt.wantUpdateRoles) > 0 {
				wantUpdates = 1
			}
			if updateCalls != wantUpdates {
				t.Fatalf("update calls = %d, want %d", updateCalls, wantUpdates)
			}
		})
	}
}

func TestGrantLocalOperatorRejectsUnverifiedIdentityAndArbitraryRole(t *testing.T) {
	client := &http.Client{Transport: testRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP request made for invalid input")
		return nil, nil
	})}
	cfg := Config{IssuerURL: "https://zitadel.example", ManagementToken: "token", ProjectID: "project-1", HTTPClient: client}
	if err := GrantLocalOperator(context.Background(), cfg, "", authidentity.AuthenticatedIdentity{}); err == nil {
		t.Fatal("GrantLocalOperator accepted a blank identity")
	}
	if err := GrantLocalOperator(context.Background(), cfg, "platform_admin", authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1"}); err == nil {
		t.Fatal("GrantLocalOperator accepted an arbitrary additional role")
	}
}

func TestProvisionerErrorsDoNotEchoTokensOrProviderSecrets(t *testing.T) {
	const managementToken = "management-token-secret"
	const providerSecret = "returned-client-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"` + providerSecret + `"}`))
	}))
	defer server.Close()

	_, err := ProvisionLocalApplications(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: managementToken, ProjectID: "project-1",
	}, LocalApplicationConfig{APIName: "ListingKit Local API", OIDCName: "ListingKit Local OIDC", RedirectURIs: []string{"http://localhost:3000/callback"}, PostLogoutRedirectURIs: []string{"http://localhost:3000"}})
	if err == nil {
		t.Fatal("ProvisionLocalApplications returned nil error")
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		for _, secret := range []string{managementToken, providerSecret} {
			if strings.Contains(current.Error(), secret) {
				t.Fatalf("error leaked %q: %v", secret, current)
			}
		}
	}
}

func TestProvisionCreatesMissingRolesOnExistingProject(t *testing.T) {
	var createdRoles []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{
				"result": []map[string]any{
					{"id": "project-1", "name": "ListingKit"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles/_search":
			writeJSON(t, w, map[string]any{
				"result": []map[string]any{
					{"key": "listingkit_viewer"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/project-1/roles":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create role body: %v", err)
			}
			createdRoles = append(createdRoles, body["roleKey"])
			writeJSON(t, w, map[string]any{"id": body["roleKey"]})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
	})
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	if result.ProjectID != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", result.ProjectID)
	}
	if strings.Join(createdRoles, ",") != "listingkit_operator,listingkit_admin,platform_admin" {
		t.Fatalf("created roles = %#v", createdRoles)
	}
	if len(result.Roles) != 4 {
		t.Fatalf("roles len = %d, want 4", len(result.Roles))
	}
	if !result.Roles[0].Existed || result.Roles[1].Existed {
		t.Fatalf("unexpected role statuses: %#v", result.Roles)
	}
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:role:listingkit_admin") {
		t.Fatalf("recommended scopes missing listingkit_admin: %#v", result.RecommendedScopes)
	}
}

func TestProvisionFailsWhenProjectIsMissingAndCreateProjectIsFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
	})
	if err == nil || !strings.Contains(err.Error(), "project ListingKit not found") {
		t.Fatalf("error = %v, want missing project error", err)
	}
}

func TestProvisionCreatesProjectWhenEnabled(t *testing.T) {
	var createProjectBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects":
			if err := json.NewDecoder(r.Body).Decode(&createProjectBody); err != nil {
				t.Fatalf("decode create project body: %v", err)
			}
			writeJSON(t, w, map[string]any{"id": "new-project"})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/new-project/roles/_search":
			writeJSON(t, w, map[string]any{"result": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/management/v1/projects/new-project/roles":
			writeJSON(t, w, map[string]any{"id": "role"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := Provision(context.Background(), Config{
		IssuerURL:       server.URL,
		ManagementToken: "token",
		ProjectName:     "ListingKit",
		CreateProject:   true,
	})
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}
	if result.ProjectID != "new-project" {
		t.Fatalf("ProjectID = %q, want new-project", result.ProjectID)
	}
	if createProjectBody["name"] != "ListingKit" {
		t.Fatalf("create project name = %#v", createProjectBody["name"])
	}
	if createProjectBody["projectRoleAssertion"] != true {
		t.Fatalf("projectRoleAssertion = %#v, want true", createProjectBody["projectRoleAssertion"])
	}
	if createProjectBody["projectRoleCheck"] != true {
		t.Fatalf("projectRoleCheck = %#v, want true", createProjectBody["projectRoleCheck"])
	}
}

func requireAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("Authorization = %q, want Bearer token", got)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func defaultRoleResponses() []map[string]any {
	roles := DefaultRoles()
	result := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		result = append(result, map[string]any{
			"key":         role.Key,
			"displayName": role.DisplayName,
			"group":       role.Group,
		})
	}
	return result
}

func assertStringSlice(t *testing.T, got any, want []string) {
	t.Helper()
	var gotStrings []string
	switch values := got.(type) {
	case []string:
		gotStrings = append([]string(nil), values...)
	case []any:
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				t.Fatalf("slice contains non-string value %#v", value)
			}
			gotStrings = append(gotStrings, text)
		}
	default:
		t.Fatalf("value = %#v, want []string", got)
	}
	if len(gotStrings) != len(want) {
		t.Fatalf("slice = %#v, want %#v", gotStrings, want)
	}
	for index := range want {
		if gotStrings[index] != want[index] {
			t.Fatalf("slice = %#v, want %#v", gotStrings, want)
		}
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
