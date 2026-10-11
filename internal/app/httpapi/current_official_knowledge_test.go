package httpapi

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/knowledge/official"
	officialhttp "task-processor/internal/knowledge/official/httpapi"
	"testing"
)

func TestOfficialKnowledgeExactRouteAdmission(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := officialhttp.Routes(&officialhttp.Handler{})
	all := append(base, routes...)
	validate := func(r []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{OfficialKnowledge: enabled})
	}
	require.NoError(t, validate(all, true))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	for i := range routes {
		for _, mutate := range []func(*httproute.Descriptor){
			func(d *httproute.Descriptor) { d.Permission = "" }, func(d *httproute.Descriptor) {
				d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
			func(d *httproute.Descriptor) { d.RejectUnreadRequestBody = false }, func(d *httproute.Descriptor) { d.RequestTimeout = 0 },
			func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyVerifiedIdentity },
		} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
}

type officialUnreadBody struct{}

type officialRolePolicy struct{ enabled bool }

func (p *officialRolePolicy) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	if p.enabled && org == "B" {
		for _, key := range keys {
			result[key] = []string{"knowledge"}
		}
	}
	return result, nil
}

func (officialUnreadBody) Read([]byte) (int, error) { panic("official GET body must not be read") }
func (officialUnreadBody) Close() error             { return nil }
func TestOfficialKnowledgeUsesLiveAccessWithoutEnterpriseStorage(t *testing.T) {
	f := newAccountFixture(t)
	cfg := &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{IssuerURL: f.provider.URL, ClientID: "fixture-client", ClientSecret: "fixture-secret", ProjectID: "project", AuthorizationAPIURL: f.provider.URL}}}
	auth, err := buildWorkbenchContextModule(cfg, logrus.New(), defaultWorkbenchContextFactories())
	require.NoError(t, err)
	catalog, err := official.NewEmbeddedCatalog()
	require.NoError(t, err)
	handler, err := officialhttp.NewHandler(catalog)
	require.NoError(t, err)
	authorization, err := buildRouteAuthorization(cfg)
	require.NoError(t, err)
	policy := &officialRolePolicy{enabled: true}
	authorization.authorizer.SetRolePolicyReader(policy)
	server := buildCurrentApplicationHTTPServer(officialhttp.Routes(handler), authorization.withWorkbench(*auth.authDependencies))
	read := func(path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Requested-Organization-ID", "B")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	require.Equal(t, 401, read(officialhttp.Base, "").Code)
	require.Equal(t, 403, read(officialhttp.Base, "u1").Code)
	f.role.Store("listingkit_operator")
	require.Equal(t, 403, read(officialhttp.Base, "u1").Code, "retired static operator is not a current enterprise role")
	f.role.Store(authz.EnterpriseRoleKey("B", 1))
	response := read(officialhttp.Base, "u1")
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Header().Get("Cache-Control"), "no-store")
	var list struct {
		Items []official.Summary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	require.Equal(t, 200, read(officialhttp.Base+"/ai-commerce-guide/revisions/1", "u1").Code)
	require.Equal(t, 404, read(officialhttp.Base+"/ai-commerce-guide/revisions/2", "u1").Code)
	require.Equal(t, 400, read(officialhttp.Base+"?organization=other", "u1").Code)
	r := httptest.NewRequest(http.MethodGet, officialhttp.Base, nil)
	r.Header.Set("Authorization", "Bearer u1")
	r.Header.Set("X-Requested-Organization-ID", "B")
	r.Body = officialUnreadBody{}
	r.ContentLength = 1
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, r)
	require.Equal(t, 400, w.Code)
	policy.enabled = false
	require.Equal(t, 403, read(officialhttp.Base+"/ai-commerce-guide/revisions/1", "u1").Code)
	policy.enabled = true
	f.revoked.Store(true)
	require.Equal(t, 403, read(officialhttp.Base, "u1").Code)
}
