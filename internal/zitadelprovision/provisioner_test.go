package zitadelprovision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
		RedirectURIs:           []string{"http://localhost:3000/api/auth/callback/zitadel"},
		PostLogoutRedirectURIs: []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("ProvisionLocalApplications returned error: %v", err)
	}

	if apiBody["name"] != "ListingKit Local API" || apiBody["authMethodType"] != "API_AUTH_METHOD_TYPE_BASIC" {
		t.Fatalf("API app body = %#v", apiBody)
	}
	assertStringSlice(t, oidcBody["redirectUris"], []string{"http://localhost:3000/api/auth/callback/zitadel"})
	assertStringSlice(t, oidcBody["postLogoutRedirectUris"], []string{"http://localhost:3000"})
	if oidcBody["name"] != "ListingKit Local OIDC" ||
		oidcBody["appType"] != "OIDC_APP_TYPE_WEB" ||
		oidcBody["authMethodType"] != "OIDC_AUTH_METHOD_TYPE_BASIC" ||
		oidcBody["accessTokenType"] != "OIDC_TOKEN_TYPE_BEARER" ||
		oidcBody["accessTokenRoleAssertion"] != true ||
		oidcBody["idTokenRoleAssertion"] != true ||
		oidcBody["devMode"] != true {
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
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:id:project-1:aud") ||
		!contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:project-1:roles") {
		t.Fatalf("recommended scopes = %#v", result.RecommendedScopes)
	}
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:id:zitadel:aud") ||
		!contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:id:project-1:aud") ||
		!contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:project-1:roles") {
		t.Fatalf("recommended scopes must authorize the ZITADEL read API and target the ListingKit project: %#v", result.RecommendedScopes)
	}
}

func TestProvisionLocalApplicationsReusesAppsByStableName(t *testing.T) {
	createCalls := 0
	updateCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/management/v1/projects/project-1/roles/_search":
			writeJSON(t, w, map[string]any{"result": defaultRoleResponses()})
		case "/management/v1/projects/project-1/apps/_search":
			writeJSON(t, w, map[string]any{"result": []map[string]any{
				{"id": "api-app-1", "name": "ListingKit Local API", "apiConfig": map[string]any{"clientId": "api-client-1"}},
				{"id": "oidc-app-1", "name": "ListingKit Local OIDC", "oidcConfig": map[string]any{"clientId": "oidc-client-1"}},
			}})
		case "/management/v1/projects/project-1/apps/api-app-1":
			writeJSON(t, w, map[string]any{"app": map[string]any{
				"id": "api-app-1", "name": "ListingKit Local API", "apiConfig": map[string]any{
					"clientId": "api-client-1", "clientSecret": "", "authMethodType": "API_AUTH_METHOD_TYPE_BASIC",
				},
			}})
		case "/management/v1/projects/project-1/apps/oidc-app-1":
			writeJSON(t, w, map[string]any{"app": map[string]any{
				"id": "oidc-app-1", "name": "ListingKit Local OIDC", "oidcConfig": map[string]any{
					"clientId": "oidc-client-1", "redirectUris": []string{"http://localhost:3000/api/zitadel-auth/callback"},
					"responseTypes": []string{"OIDC_RESPONSE_TYPE_CODE"}, "grantTypes": []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"},
					"appType": "OIDC_APP_TYPE_USER_AGENT", "authMethodType": "OIDC_AUTH_METHOD_TYPE_NONE",
					"postLogoutRedirectUris": []string{"http://localhost:3000"}, "accessTokenRoleAssertion": true,
				},
			}})
		case "/management/v1/projects/project-1/apps/oidc-app-1/oidc_config":
			if r.Method != http.MethodPut {
				t.Fatalf("OIDC update method = %s, want PUT", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode OIDC update body: %v", err)
			}
			if !reflect.DeepEqual(body["redirectUris"], []any{"http://localhost:3000/api/auth/callback/zitadel"}) {
				t.Fatalf("OIDC update body = %#v", body)
			}
			if body["devMode"] != true {
				t.Fatalf("OIDC update devMode = %#v, want true", body["devMode"])
			}
			if body["idTokenRoleAssertion"] != true {
				t.Fatalf("OIDC update idTokenRoleAssertion = %#v, want true", body["idTokenRoleAssertion"])
			}
			if body["appType"] != "OIDC_APP_TYPE_WEB" || body["authMethodType"] != "OIDC_AUTH_METHOD_TYPE_BASIC" {
				t.Fatalf("OIDC update authentication contract = %#v", body)
			}
			updateCalls++
			writeJSON(t, w, map[string]any{"details": map[string]any{"sequence": "2"}})
		case "/management/v1/projects/project-1/apps/oidc-app-1/oidc_config/_generate_client_secret":
			writeJSON(t, w, map[string]any{"clientSecret": "rotated-oidc-secret"})
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
		RedirectURIs:           []string{"http://localhost:3000/api/auth/callback/zitadel"},
		PostLogoutRedirectURIs: []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("ProvisionLocalApplications returned error: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", createCalls)
	}
	if updateCalls != 1 {
		t.Fatalf("OIDC update calls = %d, want 1", updateCalls)
	}
	if result.APIAppID != "api-app-1" || result.APIClientID != "api-client-1" || result.APIClientSecret != "" ||
		result.OIDCAppID != "oidc-app-1" || result.OIDCClientID != "oidc-client-1" || result.OIDCClientSecret != "rotated-oidc-secret" {
		t.Fatalf("result = %#v", result)
	}
}

func TestProvisionLocalApplicationsValidatesBasicAndRotatesMissingSecrets(t *testing.T) {
	for _, tt := range []struct {
		name       string
		authMethod string
	}{
		{name: "rotates basic", authMethod: "API_AUTH_METHOD_TYPE_BASIC"},
		{name: "proves omitted basic by rotation", authMethod: ""},
		{name: "repairs private key", authMethod: "API_AUTH_METHOD_TYPE_PRIVATE_KEY_JWT"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rotated := false
			currentAuthMethod := tt.authMethod
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requireAuth(t, r)
				switch r.URL.Path {
				case "/management/v1/projects/project-1/roles/_search":
					writeJSON(t, w, map[string]any{"result": defaultRoleResponses()})
				case "/management/v1/projects/project-1/apps/_search":
					writeJSON(t, w, map[string]any{"result": []map[string]any{
						{"id": "api-app-1", "name": "ListingKit Local API", "apiConfig": map[string]any{"clientId": "api-client-1"}},
						{"id": "oidc-app-1", "name": "ListingKit Local OIDC", "oidcConfig": map[string]any{"clientId": "oidc-client-1"}},
					}})
				case "/management/v1/projects/project-1/apps/api-app-1":
					writeJSON(t, w, map[string]any{"app": map[string]any{
						"id": "api-app-1", "name": "ListingKit Local API",
						"apiConfig": map[string]any{"clientId": "api-client-1", "authMethodType": currentAuthMethod},
					}})
				case "/management/v1/projects/project-1/apps/api-app-1/api_config":
					if r.Method != http.MethodPut {
						t.Fatalf("API config method = %s, want PUT", r.Method)
					}
					currentAuthMethod = "API_AUTH_METHOD_TYPE_BASIC"
					writeJSON(t, w, map[string]any{"details": map[string]any{"sequence": "2"}})
				case "/management/v1/projects/project-1/apps/api-app-1/api_config/_generate_client_secret":
					rotated = true
					writeJSON(t, w, map[string]any{"clientSecret": "rotated-api-secret"})
				case "/management/v1/projects/project-1/apps/oidc-app-1/oidc_config/_generate_client_secret":
					writeJSON(t, w, map[string]any{"clientSecret": "rotated-oidc-secret"})
				case "/management/v1/projects/project-1/apps/oidc-app-1":
					devMode := true
					_ = devMode
					writeJSON(t, w, map[string]any{"app": map[string]any{
						"id": "oidc-app-1", "name": "ListingKit Local OIDC",
						"oidcConfig": map[string]any{
							"clientId":               "oidc-client-1",
							"redirectUris":           []string{"http://localhost:3000/api/auth/callback/zitadel"},
							"postLogoutRedirectUris": []string{"http://localhost:3000"},
							"responseTypes":          []string{"OIDC_RESPONSE_TYPE_CODE"},
							"grantTypes":             []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"},
							// ZITADEL v4 may omit the default Web/Basic enum values.
							"appType":                  "",
							"authMethodType":           "",
							"accessTokenType":          "OIDC_TOKEN_TYPE_BEARER",
							"devMode":                  true,
							"accessTokenRoleAssertion": true,
							"idTokenRoleAssertion":     true,
						},
					}})
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			result, err := ProvisionLocalApplications(context.Background(), Config{
				IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1",
			}, LocalApplicationConfig{
				APIName: "ListingKit Local API", OIDCName: "ListingKit Local OIDC",
				RedirectURIs:           []string{"http://localhost:3000/api/auth/callback/zitadel"},
				PostLogoutRedirectURIs: []string{"http://localhost:3000"},
				RotateAPIClientSecret:  true,
			})
			if err != nil {
				t.Fatalf("ProvisionLocalApplications() error = %v", err)
			}
			if !rotated || result.APIClientSecret != "rotated-api-secret" {
				t.Fatalf("rotation result = %#v, rotated=%v", result, rotated)
			}
		})
	}
}

func TestProvisionLocalApplicationsRejectsNonLocalIssuerOrRedirect(t *testing.T) {
	base := Config{IssuerURL: "https://zitadel.example.com", ManagementToken: "token", ProjectID: "project-1"}
	appCfg := LocalApplicationConfig{
		APIName: "ListingKit Local API", OIDCName: "ListingKit Local OIDC",
		RedirectURIs: []string{"http://localhost:3000/api/auth/callback/zitadel"}, PostLogoutRedirectURIs: []string{"http://localhost:3000"},
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
	if !contains(result.RecommendedScopes, "urn:zitadel:iam:org:project:project-1:roles") {
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

func TestProvisionLocalMultiOrganizationAcceptanceNormalizesAndCreatesOfficialModel(t *testing.T) {
	type organization struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	}
	type projectGrant struct {
		ProjectID             string   `json:"projectId"`
		GrantedOrganizationID string   `json:"grantedOrganizationId"`
		GrantedRoleKeys       []string `json:"grantedRoleKeys"`
		State                 string   `json:"state"`
	}
	type authorization struct {
		ID           string
		UserID       string
		ProjectID    string
		Organization string
		RoleKeys     []string
		State        string
	}

	var organizations []organization
	var projectGrants []projectGrant
	var authorizations []authorization
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/v2/organizations/_search":
			writeJSON(t, w, map[string]any{"result": organizations})
		case "/v2/organizations":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			name, _ := body["name"].(string)
			organizationID, _ := body["organizationId"].(string)
			if body["orgId"] != organizationID {
				t.Fatalf("stable organization ids = %#v/%#v", body["organizationId"], body["orgId"])
			}
			created := organization{ID: organizationID, Name: name, State: "ORGANIZATION_STATE_ACTIVE"}
			organizations = append(organizations, created)
			writeJSON(t, w, map[string]any{"organizationId": created.ID})
		case "/zitadel.project.v2.ProjectService/ListProjectGrants":
			if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
				t.Fatalf("Connect-Protocol-Version = %q, want 1", got)
			}
			writeJSON(t, w, map[string]any{"projectGrants": projectGrants})
		case "/zitadel.project.v2.ProjectService/CreateProjectGrant":
			if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
				t.Fatalf("Connect-Protocol-Version = %q, want 1", got)
			}
			var body struct {
				ProjectID             string   `json:"projectId"`
				GrantedOrganizationID string   `json:"grantedOrganizationId"`
				RoleKeys              []string `json:"roleKeys"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			projectGrants = append(projectGrants, projectGrant{
				ProjectID:             body.ProjectID,
				GrantedOrganizationID: body.GrantedOrganizationID,
				GrantedRoleKeys:       body.RoleKeys,
				State:                 "PROJECT_GRANT_STATE_ACTIVE",
			})
			writeJSON(t, w, map[string]any{"creationDate": "2026-08-30T00:00:00Z"})
		case "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			response := make([]map[string]any, 0, len(authorizations))
			for _, item := range authorizations {
				roles := make([]map[string]any, 0, len(item.RoleKeys))
				for _, roleKey := range item.RoleKeys {
					roles = append(roles, map[string]any{"key": roleKey})
				}
				response = append(response, map[string]any{
					"id":           item.ID,
					"user":         map[string]any{"id": item.UserID},
					"project":      map[string]any{"id": item.ProjectID},
					"organization": map[string]any{"id": item.Organization},
					"roles":        roles,
					"state":        item.State,
				})
			}
			writeJSON(t, w, map[string]any{"authorizations": response})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			var body struct {
				UserID         string   `json:"userId"`
				ProjectID      string   `json:"projectId"`
				OrganizationID string   `json:"organizationId"`
				RoleKeys       []string `json:"roleKeys"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			authorizations = append(authorizations, authorization{
				ID:           fmt.Sprintf("authorization-%d", len(authorizations)+1),
				UserID:       body.UserID,
				ProjectID:    body.ProjectID,
				Organization: body.OrganizationID,
				RoleKeys:     body.RoleKeys,
				State:        "STATE_ACTIVE",
			})
			writeJSON(t, w, map[string]any{"id": authorizations[len(authorizations)-1].ID})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	spec := MultiOrganizationAcceptanceSpec{
		UserID: " user-1 ",
		Organizations: []AcceptanceOrganizationSpec{
			{Name: " Acceptance Organization A ", RoleKeys: []string{" listingkit_admin ", "listingkit_admin", ""}},
			{Name: " Acceptance Organization B ", RoleKeys: []string{" listingkit_viewer ", "listingkit_viewer"}},
		},
	}
	result, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: " project-1 ", AcceptanceOrganizationIDs: []string{"org-1", "org-2"},
	}, spec)
	if err != nil {
		t.Fatalf("ProvisionLocalMultiOrganizationAcceptance() error = %v", err)
	}

	want := MultiOrganizationAcceptanceResult{
		UserID: "user-1", ProjectID: "project-1",
		Organizations: []AcceptanceOrganizationResult{
			{OrganizationID: "org-1", OrganizationName: "Acceptance Organization A", RoleKeys: []string{"listingkit_admin"}},
			{OrganizationID: "org-2", OrganizationName: "Acceptance Organization B", RoleKeys: []string{"listingkit_viewer"}},
		},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}
	if len(projectGrants) != 2 || len(authorizations) != 2 {
		t.Fatalf("created project grants/authorizations = %d/%d, want 2/2", len(projectGrants), len(authorizations))
	}
	for index := range want.Organizations {
		if projectGrants[index].ProjectID != "project-1" || projectGrants[index].GrantedOrganizationID != want.Organizations[index].OrganizationID {
			t.Fatalf("project grant %d = %#v", index, projectGrants[index])
		}
		if !reflect.DeepEqual(projectGrants[index].GrantedRoleKeys, want.Organizations[index].RoleKeys) {
			t.Fatalf("project grant roles %d = %#v", index, projectGrants[index].GrantedRoleKeys)
		}
		if authorizations[index].UserID != "user-1" || authorizations[index].ProjectID != "project-1" || authorizations[index].Organization != want.Organizations[index].OrganizationID {
			t.Fatalf("authorization %d = %#v", index, authorizations[index])
		}
		if !reflect.DeepEqual(authorizations[index].RoleKeys, want.Organizations[index].RoleKeys) {
			t.Fatalf("authorization roles %d = %#v", index, authorizations[index].RoleKeys)
		}
	}

	beforeOrganizations := len(organizations)
	beforeProjectGrants := len(projectGrants)
	beforeAuthorizations := len(authorizations)
	second, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1", AcceptanceOrganizationIDs: []string{"org-1", "org-2"},
	}, spec)
	if err != nil {
		t.Fatalf("second ProvisionLocalMultiOrganizationAcceptance() error = %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("second result = %#v, want %#v", second, want)
	}
	if len(organizations) != beforeOrganizations || len(projectGrants) != beforeProjectGrants || len(authorizations) != beforeAuthorizations {
		t.Fatalf("rerun created resources: organizations=%d grants=%d authorizations=%d", len(organizations)-beforeOrganizations, len(projectGrants)-beforeProjectGrants, len(authorizations)-beforeAuthorizations)
	}
}

func TestProvisionLocalMultiOrganizationAcceptanceFailsClosedOnInvalidSpec(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		spec MultiOrganizationAcceptanceSpec
	}{
		{
			name: "blank project id",
			cfg:  Config{IssuerURL: "http://localhost:8080", ManagementToken: "token"},
			spec: MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{{Name: "A", RoleKeys: []string{"admin"}}, {Name: "B", RoleKeys: []string{"viewer"}}}},
		},
		{
			name: "blank user id",
			cfg:  Config{IssuerURL: "http://localhost:8080", ManagementToken: "token", ProjectID: "project-1"},
			spec: MultiOrganizationAcceptanceSpec{Organizations: []AcceptanceOrganizationSpec{{Name: "A", RoleKeys: []string{"admin"}}, {Name: "B", RoleKeys: []string{"viewer"}}}},
		},
		{
			name: "one organization",
			cfg:  Config{IssuerURL: "http://localhost:8080", ManagementToken: "token", ProjectID: "project-1"},
			spec: MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{{Name: "A", RoleKeys: []string{"admin"}}}},
		},
		{
			name: "duplicate normalized organizations",
			cfg:  Config{IssuerURL: "http://localhost:8080", ManagementToken: "token", ProjectID: "project-1"},
			spec: MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{{Name: " A ", RoleKeys: []string{"admin"}}, {Name: "A", RoleKeys: []string{"viewer"}}}},
		},
		{
			name: "remote issuer",
			cfg:  Config{IssuerURL: "https://identity.example.com", ManagementToken: "token", ProjectID: "project-1", AcceptanceOrganizationIDs: []string{"org-1", "org-2"}},
			spec: MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{{Name: "A", RoleKeys: []string{"admin"}}, {Name: "B", RoleKeys: []string{"viewer"}}}},
		},
		{
			name: "missing stable acceptance organization ids",
			cfg:  Config{IssuerURL: "http://localhost:8080", ManagementToken: "token", ProjectID: "project-1"},
			spec: MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{{Name: "A", RoleKeys: []string{"admin"}}, {Name: "B", RoleKeys: []string{"viewer"}}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), test.cfg, test.spec); err == nil {
				t.Fatal("ProvisionLocalMultiOrganizationAcceptance() error = nil")
			}
		})
	}
}

func TestProvisionLocalMultiOrganizationAcceptanceErrorsAndResultsAreSecretSafe(t *testing.T) {
	const managementSecret = "management-secret-that-must-not-appear"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider-response-secret-that-must-not-appear", http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: managementSecret, ProjectID: "project-1", AcceptanceOrganizationIDs: []string{"org-1", "org-2"},
	}, MultiOrganizationAcceptanceSpec{
		UserID: "user-1",
		Organizations: []AcceptanceOrganizationSpec{
			{Name: "A", RoleKeys: []string{"listingkit_admin"}},
			{Name: "B", RoleKeys: []string{"listingkit_viewer"}},
		},
	})
	if err == nil {
		t.Fatal("ProvisionLocalMultiOrganizationAcceptance() error = nil")
	}
	formatted := fmt.Sprintf("%+v %v", result, err)
	for _, secret := range []string{managementSecret, "provider-response-secret-that-must-not-appear"} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("result/error leaked secret %q: %s", secret, formatted)
		}
	}
}

func TestProvisionLocalMultiOrganizationAcceptanceRejectsSameNameWithUnmanagedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		if r.URL.Path != "/v2/organizations/_search" {
			t.Fatalf("unexpected mutation after unmanaged name collision: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		queries, _ := body["queries"].([]any)
		if len(queries) != 1 {
			t.Fatalf("organization queries = %#v, want one exact query", body["queries"])
		}
		query, _ := queries[0].(map[string]any)
		if _, byID := query["idQuery"]; byID {
			writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 0}, "result": []any{}})
			return
		}
		nameQuery, byName := query["nameQuery"].(map[string]any)
		if !byName || nameQuery["name"] != "Acceptance Organization A" || nameQuery["method"] != "TEXT_QUERY_METHOD_EQUALS" {
			t.Fatalf("organization name query = %#v", query)
		}
		writeJSON(t, w, map[string]any{
			"details": map[string]any{"totalResult": 1},
			"result":  []map[string]any{{"id": "unmanaged-org", "name": "Acceptance Organization A", "state": "ORGANIZATION_STATE_ACTIVE"}},
		})
	}))
	defer server.Close()

	_, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1", AcceptanceOrganizationIDs: []string{"managed-org-a", "managed-org-b"},
	}, MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{
		{Name: "Acceptance Organization A", RoleKeys: []string{"listingkit_admin"}},
		{Name: "Acceptance Organization B", RoleKeys: []string{"listingkit_viewer"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "same name") {
		t.Fatalf("ProvisionLocalMultiOrganizationAcceptance() error = %v, want unmanaged same-name rejection", err)
	}
}

func TestProvisionLocalMultiOrganizationAcceptanceActivatesAndReadsBackExactState(t *testing.T) {
	organizationIDs := []string{"managed-org-a", "managed-org-b"}
	names := map[string]string{"managed-org-a": "Acceptance Organization A", "managed-org-b": "Acceptance Organization B"}
	wantRoles := map[string][]string{"managed-org-a": {"listingkit_admin"}, "managed-org-b": {"listingkit_viewer"}}
	organizationStates := map[string]string{"managed-org-a": "ORGANIZATION_STATE_INACTIVE", "managed-org-b": "ORGANIZATION_STATE_INACTIVE"}
	grantStates := map[string]string{"managed-org-a": "PROJECT_GRANT_STATE_INACTIVE", "managed-org-b": "PROJECT_GRANT_STATE_INACTIVE"}
	grantRoles := map[string][]string{"managed-org-a": {"old_role"}, "managed-org-b": {"old_role"}}
	authorizationStates := map[string]string{"managed-org-a": "STATE_INACTIVE", "managed-org-b": "STATE_INACTIVE"}
	authorizationRoles := map[string][]string{"managed-org-a": {"old_role"}, "managed-org-b": {"old_role"}}
	listCounts := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch {
		case r.URL.Path == "/v2/organizations/_search":
			body := decodeTestJSONMap(t, r)
			assertTestPagination(t, body["query"], "ORGANIZATION_FIELD_NAME_NAME", body["sortingColumn"])
			organizationID := organizationIDFromQueries(t, body["queries"])
			listCounts["organization:"+organizationID]++
			writeJSON(t, w, map[string]any{
				"details": map[string]any{"totalResult": 1},
				"result":  []map[string]any{{"id": organizationID, "name": names[organizationID], "state": organizationStates[organizationID]}},
			})
		case strings.HasPrefix(r.URL.Path, "/v2/organizations/") && strings.HasSuffix(r.URL.Path, "/activate"):
			organizationID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/organizations/"), "/activate")
			organizationStates[organizationID] = "ORGANIZATION_STATE_ACTIVE"
			writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
		case r.URL.Path == "/zitadel.project.v2.ProjectService/ListProjectGrants":
			body := decodeTestJSONMap(t, r)
			assertTestPagination(t, body["pagination"], "PROJECT_GRANT_FIELD_NAME_CREATION_DATE", body["sortingColumn"])
			organizationID := projectGrantOrganizationFilter(t, body["filters"], "project-1")
			listCounts["grant:"+organizationID]++
			writeJSON(t, w, map[string]any{
				"pagination": map[string]any{"totalResult": 1, "appliedLimit": 100},
				"projectGrants": []map[string]any{{
					"projectId": "project-1", "grantedOrganizationId": organizationID,
					"grantedRoleKeys": grantRoles[organizationID], "state": grantStates[organizationID],
				}},
			})
		case r.URL.Path == "/zitadel.project.v2.ProjectService/UpdateProjectGrant":
			body := decodeTestJSONMap(t, r)
			organizationID, _ := body["grantedOrganizationId"].(string)
			grantRoles[organizationID] = testStringSlice(t, body["roleKeys"])
			writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
		case r.URL.Path == "/zitadel.project.v2.ProjectService/ActivateProjectGrant":
			body := decodeTestJSONMap(t, r)
			organizationID, _ := body["grantedOrganizationId"].(string)
			grantStates[organizationID] = "PROJECT_GRANT_STATE_ACTIVE"
			writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
		case r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			body := decodeTestJSONMap(t, r)
			assertTestPagination(t, body["pagination"], "AUTHORIZATION_FIELD_NAME_ID", body["sortingColumn"])
			organizationID := authorizationOrganizationFilter(t, body["filters"], "user-1", "project-1")
			listCounts["authorization:"+organizationID]++
			roles := make([]map[string]any, 0, len(authorizationRoles[organizationID]))
			for _, roleKey := range authorizationRoles[organizationID] {
				roles = append(roles, map[string]any{"key": roleKey})
			}
			writeJSON(t, w, map[string]any{
				"pagination": map[string]any{"totalResult": 1, "appliedLimit": 100},
				"authorizations": []map[string]any{{
					"id": "authorization-" + organizationID, "state": authorizationStates[organizationID],
					"user": map[string]any{"id": "user-1"}, "project": map[string]any{"id": "project-1"},
					"organization": map[string]any{"id": organizationID}, "roles": roles,
				}},
			})
		case r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/UpdateAuthorization":
			body := decodeTestJSONMap(t, r)
			authorizationID, _ := body["id"].(string)
			organizationID := strings.TrimPrefix(authorizationID, "authorization-")
			authorizationRoles[organizationID] = testStringSlice(t, body["roleKeys"])
			writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
		case r.URL.Path == "/zitadel.authorization.v2.AuthorizationService/ActivateAuthorization":
			body := decodeTestJSONMap(t, r)
			authorizationID, _ := body["id"].(string)
			organizationID := strings.TrimPrefix(authorizationID, "authorization-")
			authorizationStates[organizationID] = "STATE_ACTIVE"
			writeJSON(t, w, map[string]any{"changeDate": "2026-08-30T00:00:00Z"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1", AcceptanceOrganizationIDs: organizationIDs,
	}, MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{
		{Name: names[organizationIDs[0]], RoleKeys: wantRoles[organizationIDs[0]]},
		{Name: names[organizationIDs[1]], RoleKeys: wantRoles[organizationIDs[1]]},
	}})
	if err != nil {
		t.Fatalf("ProvisionLocalMultiOrganizationAcceptance() error = %v", err)
	}
	if len(result.Organizations) != 2 {
		t.Fatalf("result organizations = %d, want 2", len(result.Organizations))
	}
	for _, organizationID := range organizationIDs {
		if organizationStates[organizationID] != "ORGANIZATION_STATE_ACTIVE" || grantStates[organizationID] != "PROJECT_GRANT_STATE_ACTIVE" || authorizationStates[organizationID] != "STATE_ACTIVE" {
			t.Fatalf("final states for %s = %s/%s/%s", organizationID, organizationStates[organizationID], grantStates[organizationID], authorizationStates[organizationID])
		}
		if !reflect.DeepEqual(grantRoles[organizationID], wantRoles[organizationID]) || !reflect.DeepEqual(authorizationRoles[organizationID], wantRoles[organizationID]) {
			t.Fatalf("final roles for %s = %#v/%#v", organizationID, grantRoles[organizationID], authorizationRoles[organizationID])
		}
		for _, kind := range []string{"organization:", "grant:", "authorization:"} {
			if listCounts[kind+organizationID] < 2 {
				t.Fatalf("%s%s list count = %d, want write read-back", kind, organizationID, listCounts[kind+organizationID])
			}
		}
	}
}

func TestProvisionLocalMultiOrganizationAcceptanceRecoversCreateConflictsByReadBack(t *testing.T) {
	organizationIDs := []string{"managed-org-a", "managed-org-b"}
	names := map[string]string{"managed-org-a": "Acceptance Organization A", "managed-org-b": "Acceptance Organization B"}
	wantRoles := map[string][]string{"managed-org-a": {"listingkit_admin"}, "managed-org-b": {"listingkit_viewer"}}
	organizations := make(map[string]organizationRecord)
	grants := make(map[string]projectGrantRecord)
	authorizations := make(map[string]authorizationRecord)
	failAuthorizationBOnce := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/v2/organizations/_search":
			body := decodeTestJSONMap(t, r)
			queries, _ := body["queries"].([]any)
			query, _ := queries[0].(map[string]any)
			if idQuery, ok := query["idQuery"].(map[string]any); ok {
				organizationID, _ := idQuery["id"].(string)
				if organization, found := organizations[organizationID]; found {
					writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 1}, "result": []organizationRecord{organization}})
					return
				}
			} else if nameQuery, ok := query["nameQuery"].(map[string]any); ok {
				name, _ := nameQuery["name"].(string)
				for _, organization := range organizations {
					if organization.Name == name {
						writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 1}, "result": []organizationRecord{organization}})
						return
					}
				}
			}
			writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 0}, "result": []any{}})
		case "/v2/organizations":
			body := decodeTestJSONMap(t, r)
			organizationID, _ := body["organizationId"].(string)
			if body["orgId"] != organizationID || organizationID == "" {
				t.Fatalf("create organization stable ids = %#v/%#v", body["organizationId"], body["orgId"])
			}
			name, _ := body["name"].(string)
			organizations[organizationID] = organizationRecord{ID: organizationID, Name: name, State: "ORGANIZATION_STATE_ACTIVE"}
			w.WriteHeader(http.StatusConflict)
		case "/zitadel.project.v2.ProjectService/ListProjectGrants":
			body := decodeTestJSONMap(t, r)
			organizationID := projectGrantOrganizationFilter(t, body["filters"], "project-1")
			if grant, found := grants[organizationID]; found {
				writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 1}, "projectGrants": []projectGrantRecord{grant}})
				return
			}
			writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 0}, "projectGrants": []any{}})
		case "/zitadel.project.v2.ProjectService/CreateProjectGrant":
			body := decodeTestJSONMap(t, r)
			organizationID, _ := body["grantedOrganizationId"].(string)
			grants[organizationID] = projectGrantRecord{
				ProjectID: "project-1", GrantedOrganizationID: organizationID,
				GrantedRoleKeys: testStringSlice(t, body["roleKeys"]), State: "PROJECT_GRANT_STATE_ACTIVE",
			}
			w.WriteHeader(http.StatusConflict)
		case "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			body := decodeTestJSONMap(t, r)
			organizationID := authorizationOrganizationFilter(t, body["filters"], "user-1", "project-1")
			if authorization, found := authorizations[organizationID]; found {
				writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 1}, "authorizations": []authorizationRecord{authorization}})
				return
			}
			writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 0}, "authorizations": []any{}})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			body := decodeTestJSONMap(t, r)
			organizationID, _ := body["organizationId"].(string)
			if organizationID == "managed-org-b" && failAuthorizationBOnce {
				failAuthorizationBOnce = false
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			record := authorizationRecord{ID: "authorization-" + organizationID, State: "STATE_ACTIVE"}
			record.User.ID = "user-1"
			record.Project.ID = "project-1"
			record.Organization.ID = organizationID
			for _, roleKey := range testStringSlice(t, body["roleKeys"]) {
				record.Roles = append(record.Roles, struct {
					Key string `json:"key"`
				}{Key: roleKey})
			}
			authorizations[organizationID] = record
			w.WriteHeader(http.StatusConflict)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := Config{
		IssuerURL: server.URL, ManagementToken: "token", ProjectID: "project-1", AcceptanceOrganizationIDs: organizationIDs,
	}
	spec := MultiOrganizationAcceptanceSpec{UserID: "user-1", Organizations: []AcceptanceOrganizationSpec{
		{Name: names[organizationIDs[0]], RoleKeys: wantRoles[organizationIDs[0]]},
		{Name: names[organizationIDs[1]], RoleKeys: wantRoles[organizationIDs[1]]},
	}}
	if _, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), cfg, spec); err == nil {
		t.Fatal("first ProvisionLocalMultiOrganizationAcceptance() error = nil, want injected partial failure")
	}
	_, err := ProvisionLocalMultiOrganizationAcceptance(context.Background(), cfg, spec)
	if err != nil {
		t.Fatalf("ProvisionLocalMultiOrganizationAcceptance() retry after partial failure error = %v", err)
	}
}

func TestEnsureAcceptanceOrganizationHandlesConcurrentCreateConflict(t *testing.T) {
	var mu sync.Mutex
	initialIDReads := 0
	initialReadsDone := make(chan struct{})
	created := false
	createCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/v2/organizations/_search":
			body := decodeTestJSONMap(t, r)
			queries, _ := body["queries"].([]any)
			query, _ := queries[0].(map[string]any)
			if _, isID := query["idQuery"]; isID {
				mu.Lock()
				if !created && initialIDReads < 2 {
					initialIDReads++
					if initialIDReads == 2 {
						close(initialReadsDone)
					}
					mu.Unlock()
					<-initialReadsDone
					writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 0}, "result": []any{}})
					return
				}
				isCreated := created
				mu.Unlock()
				if isCreated {
					writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 1}, "result": []map[string]any{{
						"id": "managed-org-a", "name": "Acceptance Organization A", "state": "ORGANIZATION_STATE_ACTIVE",
					}}})
					return
				}
			}
			writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 0}, "result": []any{}})
		case "/v2/organizations":
			body := decodeTestJSONMap(t, r)
			if body["organizationId"] != "managed-org-a" || body["orgId"] != "managed-org-a" {
				t.Fatalf("create organization body = %#v", body)
			}
			mu.Lock()
			createCalls++
			if !created {
				created = true
				mu.Unlock()
				writeJSON(t, w, map[string]any{"organizationId": "managed-org-a"})
				return
			}
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"})
	errorsByCall := make(chan error, 2)
	for range 2 {
		go func() {
			errorsByCall <- client.ensureAcceptanceOrganization(context.Background(), "managed-org-a", "Acceptance Organization A")
		}()
	}
	for range 2 {
		if err := <-errorsByCall; err != nil {
			t.Fatalf("ensureAcceptanceOrganization() concurrent error = %v", err)
		}
	}
	if createCalls != 2 {
		t.Fatalf("create calls = %d, want one success and one conflict", createCalls)
	}
}

func TestEnsureAcceptanceOrganizationRetriesEventuallyConsistentReadBack(t *testing.T) {
	created := false
	postCreateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/v2/organizations/_search":
			body := decodeTestJSONMap(t, r)
			queries, _ := body["queries"].([]any)
			query, _ := queries[0].(map[string]any)
			if _, isID := query["idQuery"]; isID && created {
				postCreateReads++
				if postCreateReads >= 3 {
					writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 1}, "result": []map[string]any{{
						"id": "managed-org-a", "name": "Acceptance Organization A", "state": "ORGANIZATION_STATE_ACTIVE",
					}}})
					return
				}
			}
			writeJSON(t, w, map[string]any{"details": map[string]any{"totalResult": 0}, "result": []any{}})
		case "/v2/organizations":
			created = true
			writeJSON(t, w, map[string]any{"organizationId": "managed-org-a"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"})
	if err := client.ensureAcceptanceOrganization(context.Background(), "managed-org-a", "Acceptance Organization A"); err != nil {
		t.Fatalf("ensureAcceptanceOrganization() error = %v", err)
	}
	if postCreateReads != 3 {
		t.Fatalf("post-create reads = %d, want 3", postCreateReads)
	}
}

func TestEnsureProjectGrantRetriesEventuallyConsistentReadBack(t *testing.T) {
	created := false
	postCreateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/zitadel.project.v2.ProjectService/ListProjectGrants":
			if created {
				postCreateReads++
				if postCreateReads >= 3 {
					writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 1}, "projectGrants": []map[string]any{{
						"projectId": "project-1", "grantedOrganizationId": "managed-org-a",
						"grantedRoleKeys": []string{"listingkit_admin"}, "state": "PROJECT_GRANT_STATE_ACTIVE",
					}}})
					return
				}
			}
			writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 0}, "projectGrants": []any{}})
		case "/zitadel.project.v2.ProjectService/CreateProjectGrant":
			created = true
			writeJSON(t, w, map[string]any{"creationDate": "2026-08-31T00:00:00Z"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"})
	if err := client.ensureProjectGrant(context.Background(), "project-1", "managed-org-a", []string{"listingkit_admin"}); err != nil {
		t.Fatalf("ensureProjectGrant() error = %v", err)
	}
	if postCreateReads != 3 {
		t.Fatalf("post-create reads = %d, want 3", postCreateReads)
	}
}

func TestEnsureAuthorizationRetriesEventuallyConsistentReadBack(t *testing.T) {
	created := false
	postCreateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		switch r.URL.Path {
		case "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations":
			if created {
				postCreateReads++
				if postCreateReads >= 3 {
					writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 1}, "authorizations": []map[string]any{{
						"id": "authorization-a", "state": "STATE_ACTIVE", "user": map[string]any{"id": "user-1"},
						"project": map[string]any{"id": "project-1"}, "organization": map[string]any{"id": "managed-org-a"},
						"roles": []map[string]any{{"key": "listingkit_admin"}},
					}}})
					return
				}
			}
			writeJSON(t, w, map[string]any{"pagination": map[string]any{"totalResult": 0}, "authorizations": []any{}})
		case "/zitadel.authorization.v2.AuthorizationService/CreateAuthorization":
			created = true
			writeJSON(t, w, map[string]any{"id": "authorization-a"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"})
	if err := client.ensureAuthorization(context.Background(), "user-1", "project-1", "managed-org-a", []string{"listingkit_admin"}); err != nil {
		t.Fatalf("ensureAuthorization() error = %v", err)
	}
	if postCreateReads != 3 {
		t.Fatalf("post-create reads = %d, want 3", postCreateReads)
	}
}

func TestLoopbackOnlyClientRejectsAmbiguousIssuerAndResolverPollution(t *testing.T) {
	for _, raw := range []string{
		"ftp://localhost:8080", "http://user@localhost:8080", "http://localhost:8080/path",
		"http://localhost:8080?query=1", "http://localhost:8080#fragment", "http://2130706433:8080",
		"http://[::ffff:127.0.0.1]:8080", "http://localhost.:8080",
	} {
		if err := validateLocalIssuer(raw); err == nil {
			t.Fatalf("validateLocalIssuer(%q) error = nil", raw)
		}
	}
	_, err := newLoopbackOnlyHTTPClient("http://localhost:8080", func(context.Context, string, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.0.2.10")}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("newLoopbackOnlyHTTPClient() error = %v, want resolver pollution rejection", err)
	}
}

func TestLoopbackOnlyClientDoesNotVisitRedirectTarget(t *testing.T) {
	targetVisited := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetVisited <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()

	client, err := NewLoopbackOnlyHTTPClient(redirect.URL)
	if err != nil {
		t.Fatalf("NewLoopbackOnlyHTTPClient() error = %v", err)
	}
	response, err := client.Get(redirect.URL)
	if err != nil {
		t.Fatalf("loopback client GET redirect error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("redirect status = %d, want original 302 response", response.StatusCode)
	}
	select {
	case <-targetVisited:
		t.Fatal("redirect target was accessed")
	default:
	}
}

func TestLoopbackOnlyClientDialsResolverVerifiedLoopbackIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("split httptest address: %v", err)
	}

	var lookupHosts []string
	lookup := func(_ context.Context, network, host string) ([]net.IP, error) {
		if network != "ip" {
			t.Fatalf("lookup network = %q, want ip", network)
		}
		lookupHosts = append(lookupHosts, host)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	var dialAddresses []string
	realDialer := &net.Dialer{}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialAddresses = append(dialAddresses, address)
		return realDialer.DialContext(ctx, network, address)
	}

	issuer := "http://localhost:" + port
	client, err := newLoopbackOnlyHTTPClientWithDialer(issuer, lookup, dial)
	if err != nil {
		t.Fatalf("newLoopbackOnlyHTTPClientWithDialer() error = %v", err)
	}
	response, err := client.Get(issuer)
	if err != nil {
		t.Fatalf("loopback client GET error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("loopback response status = %d, want 204", response.StatusCode)
	}
	if !reflect.DeepEqual(lookupHosts, []string{"localhost", "localhost"}) {
		t.Fatalf("resolver hosts = %#v, want constructor and dial validation", lookupHosts)
	}
	wantDialAddress := net.JoinHostPort("127.0.0.1", port)
	if !reflect.DeepEqual(dialAddresses, []string{wantDialAddress}) {
		t.Fatalf("dial addresses = %#v, want only resolver-verified loopback %q", dialAddresses, wantDialAddress)
	}
}

func TestLoopbackHTTPClientHasOverallTimeoutAndHonorsRequestCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client, err := NewLoopbackOnlyHTTPClient(server.URL)
	if err != nil {
		t.Fatalf("NewLoopbackOnlyHTTPClient() error = %v", err)
	}
	if client.Timeout != 5*time.Second {
		t.Fatalf("loopback client timeout = %s, want 5s overall bound", client.Timeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	started := time.Now()
	_, err = client.Do(request)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("client.Do() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("request cancellation took %s, want under 1s", elapsed)
	}
}

func TestOrganizationPaginationContract(t *testing.T) {
	activeOrganization := func(id string) map[string]any {
		return map[string]any{"id": id, "name": "Acceptance Organization", "state": "ORGANIZATION_STATE_ACTIVE"}
	}

	t.Run("merges two pages using the observed page length as the next offset", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/v2/organizations/_search", "query", []any{
			map[string]any{"details": map[string]any{"totalResult": "2"}, "result": []any{activeOrganization("org-1")}},
			map[string]any{"details": map[string]any{"totalResult": "2"}, "result": []any{activeOrganization("org-2")}},
		})
		defer server.Close()

		records, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listOrganizations(
			context.Background(), map[string]any{"nameQuery": map[string]any{"name": "Acceptance Organization"}},
		)
		if err != nil {
			t.Fatalf("listOrganizations() error = %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("listOrganizations() records = %d, want 2", len(records))
		}
		if got := []string{records[0].ID, records[1].ID}; !reflect.DeepEqual(got, []string{"org-1", "org-2"}) {
			t.Fatalf("listOrganizations() ids = %#v, want both pages in order", got)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("organization offsets = %#v, want literal second-page offset 1", *offsets)
		}
	})

	t.Run("rejects a duplicate stable id returned on the second page", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/v2/organizations/_search", "query", []any{
			map[string]any{"details": map[string]any{"totalResult": 2}, "result": []any{activeOrganization("org-1")}},
			map[string]any{"details": map[string]any{"totalResult": 2}, "result": []any{activeOrganization("org-1")}},
		})
		defer server.Close()

		_, _, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).findOrganizationByID(context.Background(), "org-1")
		if err == nil || !strings.Contains(err.Error(), "multiple organizations") {
			t.Fatalf("findOrganizationByID() error = %v, want duplicate rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("organization duplicate offsets = %#v, want both pages", *offsets)
		}
	})

	t.Run("fails closed when an incomplete second page is empty", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/v2/organizations/_search", "query", []any{
			map[string]any{"details": map[string]any{"totalResult": 2}, "result": []any{activeOrganization("org-1")}},
			map[string]any{"details": map[string]any{"totalResult": 2}, "result": []any{}},
		})
		defer server.Close()

		_, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listOrganizations(
			context.Background(), map[string]any{"idQuery": map[string]any{"id": "org-1"}},
		)
		if err == nil || !strings.Contains(err.Error(), "made no progress") {
			t.Fatalf("listOrganizations() error = %v, want incomplete empty-page rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("organization empty-page offsets = %#v, want both pages", *offsets)
		}
	})
}

func TestProjectGrantPaginationContract(t *testing.T) {
	activeGrant := func() map[string]any {
		return map[string]any{
			"projectId": "project-1", "grantedOrganizationId": "org-1",
			"grantedRoleKeys": []string{"listingkit_admin"}, "state": "PROJECT_GRANT_STATE_ACTIVE",
		}
	}

	t.Run("merges two pages using the observed page length as the next offset", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.project.v2.ProjectService/ListProjectGrants", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": "2"}, "projectGrants": []any{activeGrant()}},
			map[string]any{"pagination": map[string]any{"totalResult": "2"}, "projectGrants": []any{activeGrant()}},
		})
		defer server.Close()

		records, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listProjectGrants(context.Background(), "project-1", "org-1")
		if err != nil {
			t.Fatalf("listProjectGrants() error = %v", err)
		}
		if len(records) != 2 || records[0].GrantedOrganizationID != "org-1" || records[1].GrantedOrganizationID != "org-1" {
			t.Fatalf("listProjectGrants() records = %#v, want both pages merged", records)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("project grant offsets = %#v, want literal second-page offset 1", *offsets)
		}
	})

	t.Run("rejects a duplicate exact candidate returned on the second page", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.project.v2.ProjectService/ListProjectGrants", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "projectGrants": []any{activeGrant()}},
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "projectGrants": []any{activeGrant()}},
		})
		defer server.Close()

		err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).ensureProjectGrant(
			context.Background(), "project-1", "org-1", []string{"listingkit_admin"},
		)
		if err == nil || !strings.Contains(err.Error(), "multiple acceptance project grants") {
			t.Fatalf("ensureProjectGrant() error = %v, want duplicate rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("project grant duplicate offsets = %#v, want both pages", *offsets)
		}
	})

	t.Run("fails closed when an incomplete second page is empty", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.project.v2.ProjectService/ListProjectGrants", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "projectGrants": []any{activeGrant()}},
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "projectGrants": []any{}},
		})
		defer server.Close()

		_, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listProjectGrants(context.Background(), "project-1", "org-1")
		if err == nil || !strings.Contains(err.Error(), "made no progress") {
			t.Fatalf("listProjectGrants() error = %v, want incomplete empty-page rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("project grant empty-page offsets = %#v, want both pages", *offsets)
		}
	})
}

func TestAuthorizationPaginationContract(t *testing.T) {
	activeAuthorization := func(id string) map[string]any {
		return map[string]any{
			"id": id, "state": "STATE_ACTIVE", "user": map[string]any{"id": "user-1"},
			"project": map[string]any{"id": "project-1"}, "organization": map[string]any{"id": "org-1"},
			"roles": []map[string]any{{"key": "listingkit_admin"}},
		}
	}

	t.Run("merges two pages using the observed page length as the next offset", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": "2"}, "authorizations": []any{activeAuthorization("authorization-1")}},
			map[string]any{"pagination": map[string]any{"totalResult": "2"}, "authorizations": []any{activeAuthorization("authorization-2")}},
		})
		defer server.Close()

		records, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listAuthorizations(context.Background(), "user-1", "project-1", "org-1")
		if err != nil {
			t.Fatalf("listAuthorizations() error = %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("listAuthorizations() records = %d, want 2", len(records))
		}
		if got := []string{records[0].ID, records[1].ID}; !reflect.DeepEqual(got, []string{"authorization-1", "authorization-2"}) {
			t.Fatalf("listAuthorizations() ids = %#v, want both pages in order", got)
		}
		if !reflect.DeepEqual(records[0].RoleKeys, []string{"listingkit_admin"}) || !reflect.DeepEqual(records[1].RoleKeys, []string{"listingkit_admin"}) {
			t.Fatalf("listAuthorizations() roles = %#v/%#v, want both pages normalized", records[0].RoleKeys, records[1].RoleKeys)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("authorization offsets = %#v, want literal second-page offset 1", *offsets)
		}
	})

	t.Run("rejects a duplicate exact candidate returned on the second page", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "authorizations": []any{activeAuthorization("authorization-1")}},
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "authorizations": []any{activeAuthorization("authorization-1")}},
		})
		defer server.Close()

		err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).ensureAuthorization(
			context.Background(), "user-1", "project-1", "org-1", []string{"listingkit_admin"},
		)
		if err == nil || !strings.Contains(err.Error(), "multiple acceptance role assignments") {
			t.Fatalf("ensureAuthorization() error = %v, want duplicate rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("authorization duplicate offsets = %#v, want both pages", *offsets)
		}
	})

	t.Run("fails closed when an incomplete second page is empty", func(t *testing.T) {
		server, offsets := newTestPagedServer(t, "/zitadel.authorization.v2.AuthorizationService/ListAuthorizations", "pagination", []any{
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "authorizations": []any{activeAuthorization("authorization-1")}},
			map[string]any{"pagination": map[string]any{"totalResult": 2}, "authorizations": []any{}},
		})
		defer server.Close()

		_, err := newClient(Config{IssuerURL: server.URL, ManagementToken: "token"}).listAuthorizations(context.Background(), "user-1", "project-1", "org-1")
		if err == nil || !strings.Contains(err.Error(), "made no progress") {
			t.Fatalf("listAuthorizations() error = %v, want incomplete empty-page rejection", err)
		}
		if !reflect.DeepEqual(*offsets, []int{0, 1}) {
			t.Fatalf("authorization empty-page offsets = %#v, want both pages", *offsets)
		}
	})
}

func newTestPagedServer(t *testing.T, path, paginationField string, responses []any) (*httptest.Server, *[]int) {
	t.Helper()
	offsets := make([]int, 0, len(responses))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != path {
			t.Errorf("paged request = %s %s, want POST %s", r.Method, r.URL.Path, path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		body := decodeTestJSONMap(t, r)
		pagination, ok := body[paginationField].(map[string]any)
		if !ok {
			t.Errorf("%s pagination = %#v", path, body[paginationField])
			http.Error(w, "missing pagination", http.StatusBadRequest)
			return
		}
		offset, ok := pagination["offset"].(float64)
		if !ok {
			t.Errorf("%s offset = %#v", path, pagination["offset"])
			http.Error(w, "missing offset", http.StatusBadRequest)
			return
		}
		page := len(offsets)
		offsets = append(offsets, int(offset))
		if page >= len(responses) {
			t.Errorf("%s requested unexpected page %d at offset %d", path, page+1, int(offset))
			http.Error(w, "unexpected extra page", http.StatusBadRequest)
			return
		}
		writeJSON(t, w, responses[page])
	}))
	return server, &offsets
}

func decodeTestJSONMap(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", r.URL.Path, err)
	}
	return body
}

func assertTestPagination(t *testing.T, pagination any, wantSorting string, sorting any) {
	t.Helper()
	values, ok := pagination.(map[string]any)
	if !ok || values["offset"] != float64(0) || values["limit"] != float64(100) || values["asc"] != true {
		t.Fatalf("pagination = %#v, want offset=0 limit=100 asc=true", pagination)
	}
	if sorting != wantSorting {
		t.Fatalf("sortingColumn = %#v, want %q", sorting, wantSorting)
	}
}

func organizationIDFromQueries(t *testing.T, value any) string {
	t.Helper()
	queries, ok := value.([]any)
	if !ok || len(queries) != 1 {
		t.Fatalf("organization queries = %#v", value)
	}
	query, _ := queries[0].(map[string]any)
	idQuery, ok := query["idQuery"].(map[string]any)
	if !ok {
		t.Fatalf("organization query = %#v, want idQuery", query)
	}
	id, _ := idQuery["id"].(string)
	return id
}

func projectGrantOrganizationFilter(t *testing.T, value any, wantProjectID string) string {
	t.Helper()
	filters, ok := value.([]any)
	if !ok || len(filters) != 2 {
		t.Fatalf("project grant filters = %#v, want project and organization", value)
	}
	var organizationID string
	var projectSeen bool
	for _, raw := range filters {
		filter, _ := raw.(map[string]any)
		if item, ok := filter["inProjectIdsFilter"].(map[string]any); ok {
			ids := testStringSlice(t, item["ids"])
			projectSeen = reflect.DeepEqual(ids, []string{wantProjectID})
		}
		if item, ok := filter["grantedOrganizationIdFilter"].(map[string]any); ok {
			organizationID, _ = item["id"].(string)
		}
	}
	if !projectSeen || organizationID == "" {
		t.Fatalf("project grant filters = %#v", value)
	}
	return organizationID
}

func authorizationOrganizationFilter(t *testing.T, value any, wantUserID, wantProjectID string) string {
	t.Helper()
	filters, ok := value.([]any)
	if !ok || len(filters) != 3 {
		t.Fatalf("authorization filters = %#v, want user, project, organization", value)
	}
	var organizationID string
	var userSeen, projectSeen bool
	for _, raw := range filters {
		filter, _ := raw.(map[string]any)
		if item, ok := filter["inUserIds"].(map[string]any); ok {
			userSeen = reflect.DeepEqual(testStringSlice(t, item["ids"]), []string{wantUserID})
		}
		if item, ok := filter["projectId"].(map[string]any); ok {
			projectSeen = item["id"] == wantProjectID
		}
		if item, ok := filter["organizationId"].(map[string]any); ok {
			organizationID, _ = item["id"].(string)
		}
	}
	if !userSeen || !projectSeen || organizationID == "" {
		t.Fatalf("authorization filters = %#v", value)
	}
	return organizationID
}

func testStringSlice(t *testing.T, value any) []string {
	t.Helper()
	values, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v, want string slice", value)
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("value item = %#v, want string", item)
		}
		result = append(result, text)
	}
	return result
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
