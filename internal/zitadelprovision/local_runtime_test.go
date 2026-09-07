package zitadelprovision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestLocalRuntimeProjectChecksRemainExplicitAcrossProvision(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "local_opt_in"}[explicit], func(t *testing.T) {
			var bodies []map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/management/v1/projects/_search":
					writeJSON(t, w, map[string]any{"result": []any{}})
				case "/management/v1/projects", "/management/v1/projects/p":
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					bodies = append(bodies, body)
					writeJSON(t, w, map[string]any{"id": "p"})
				case "/management/v1/projects/p/roles/_search":
					writeJSON(t, w, map[string]any{"result": defaultRoleResponses()})
				case "/management/v1/projects/p/apps/_search":
					writeJSON(t, w, map[string]any{"result": []any{}})
				case "/management/v1/projects/p/apps/api", "/management/v1/projects/p/apps/oidc":
					writeJSON(t, w, map[string]any{"appId": "app", "clientId": "client", "clientSecret": "secret"})
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer srv.Close()
			disabled := false
			cfg := Config{IssuerURL: srv.URL, ManagementToken: "token", CreateProject: true, HasProjectCheck: &disabled}
			if explicit {
				cfg.ProjectRoleCheck = &disabled
			}
			result, err := Provision(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ProjectID = result.ProjectID
			for range 2 {
				_, err = ProvisionLocalApplications(context.Background(), cfg, LocalApplicationConfig{APIName: "api", OIDCName: "oidc", RedirectURIs: []string{"http://localhost:3000/api/auth/callback/zitadel"}, PostLogoutRedirectURIs: []string{"http://localhost:3000"}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(bodies) != 3 {
				t.Fatalf("project writes=%d", len(bodies))
			}
			for _, b := range bodies {
				if b["projectRoleCheck"] != !explicit || b["hasProjectCheck"] != false || b["projectRoleAssertion"] != true {
					t.Fatalf("project flags=%v", b)
				}
			}
		})
	}
}

func TestLocalRuntimeOriginAndRefreshApplyToCreateAndUpdate(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Fatal(err)
		}
		assertStringSlice(t, b["redirectUris"], []string{"http://localhost:42123/api/auth/callback/zitadel"})
		assertStringSlice(t, b["grantTypes"], []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN"})
		calls++
		writeJSON(t, w, map[string]any{"appId": "app", "clientId": "client", "clientSecret": "secret"})
	}))
	defer srv.Close()
	cfg := LocalApplicationConfig{LocalOrigin: "http://localhost:42123", EnableRefreshToken: true, OIDCName: "test", RedirectURIs: []string{"http://localhost:42123/api/auth/callback/zitadel"}, PostLogoutRedirectURIs: []string{"http://localhost:42123"}}
	if err := validateLocalApplicationURLs(cfg); err != nil {
		t.Fatal(err)
	}
	c := newClient(Config{IssuerURL: srv.URL, ManagementToken: "token"})
	if _, err := c.createOIDCApplication(context.Background(), "p", cfg); err != nil {
		t.Fatal(err)
	}
	if err := c.updateOIDCApplicationConfig(context.Background(), "p", "app", cfg); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	base := cfg
	for _, origin := range []string{"http://example.com:42123", "http://localhost", "http://localhost:0", "http://localhost:65536", "http://user@localhost:42123", "http://localhost:42123/", "http://localhost:42123?q=1", "http://localhost:42123#x", "http://LOCALHOST:42123", "http://127.0.0.1:42123"} {
		cfg = base
		cfg.LocalOrigin = origin
		if err := validateLocalApplicationURLs(cfg); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
	cfg = base
	cfg.RedirectURIs = append(cfg.RedirectURIs, "http://localhost:42123/other")
	if validateLocalApplicationURLs(cfg) == nil {
		t.Fatal("extra callback accepted")
	}
	cfg = base
	cfg.PostLogoutRedirectURIs = []string{"http://localhost:42124"}
	if validateLocalApplicationURLs(cfg) == nil {
		t.Fatal("other port accepted")
	}
	cfg = base
	cfg.LocalOrigin = ""
	if validateLocalApplicationURLs(cfg) == nil {
		t.Fatal("random origin implicitly accepted")
	}
	if !reflect.DeepEqual(localOIDCGrantTypes(LocalApplicationConfig{}), []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE"}) {
		t.Fatal("changed legacy default")
	}
}
