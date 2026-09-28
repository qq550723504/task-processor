package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/workbenchcontext"
)

type resourceBalanceFixture struct {
	calls   int
	failure bool
}

func (r *resourceBalanceFixture) ReadBalances(_ context.Context, org string) (orgresource.Balances, error) {
	r.calls++
	if r.failure {
		return orgresource.Balances{}, orgresource.ErrBalanceUnavailable
	}
	return orgresource.Balances{SchemaVersion: "organization-resource-balances-v1", OrganizationID: org, ObservedAt: time.Now().UTC(), Resources: []orgresource.Balance{}}, nil
}

func TestCommercialResourcesLiveAuthorization(t *testing.T) {
	reader := &resourceBalanceFixture{}
	grants := &auditHTTPGrants{role: "listingkit_operator"}
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	module := commercialResourcesModule{reader: reader}
	server := buildIsolatedApplicationHTTPServer(module.routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authorizer}, 15*time.Second)
	get := func(org, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(body))
		req.Header.Set("X-Requested-Organization-ID", org)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, req)
		return w
	}
	w := get("B", commercialResourcesPath, "", "fixture")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"organization_id":"B"`) || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("read: %d %s", w.Code, w.Body.String())
	}
	for _, mode := range []string{"other_org", "viewer", "revoked", "unauthenticated", "query", "empty_query", "body", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			org, path, body, token := "B", commercialResourcesPath, "", "fixture"
			want := 403
			switch mode {
			case "other_org":
				org = "A"
			case "viewer":
				grants.role = "listingkit_viewer"
			case "revoked":
				grants.revoked = true
			case "unauthenticated":
				token = ""
				want = 401
			case "query":
				path += "?organization_id=A"
				want = 400
			case "empty_query":
				path += "?"
				want = 400
			case "body":
				body = "{}"
				want = 400
			case "unavailable":
				reader.failure = true
				want = 503
			}
			before := reader.calls
			w := get(org, path, body, token)
			if w.Code != want {
				t.Fatalf("%s %d %s", mode, w.Code, w.Body.String())
			}
			if mode != "unavailable" && reader.calls != before {
				t.Fatal("denied request reached resource owner")
			}
			grants.role = "listingkit_operator"
			grants.revoked = false
			reader.failure = false
		})
	}
}

func TestCommercialResourcesAdmissionAndIndependentInjection(t *testing.T) {
	if defaultCurrentApplicationFactories(context.Background()).buildCommercialResources == nil {
		t.Fatal("default resources builder missing")
	}
	for _, mode := range []string{"enabled", "error", "nil"} {
		t.Run(mode, func(t *testing.T) {
			resourceDB := &gorm.DB{}
			calls := 0
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					deps := newRouteAuthDependencies()
					return workbenchContextBuildResult{module: currentApplicationTestModule{name: "base", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercial:    func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercialResources: func(_ context.Context, db *gorm.DB) (kernelmodule.Module, error) {
					calls++
					if db != resourceDB {
						t.Fatal("wrong resource owner pool")
					}
					if mode == "error" {
						return nil, errors.New("unavailable")
					}
					if mode == "nil" {
						return nil, nil
					}
					return commercialResourcesModule{reader: &resourceBalanceFixture{}}, nil
				},
				buildCommercialBilling: func(context.Context, *gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer, *config.Config) (kernelmodule.Module, error) {
					t.Fatal("resource read requires no money DB")
					return nil, nil
				},
			}
			server, err := buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories, WithCommercialOwnerDatabase(resourceDB))
			if calls != 1 {
				t.Fatal("resource builder was not called")
			}
			if mode == "enabled" {
				if err != nil || server == nil {
					t.Fatal(err)
				}
			} else if err == nil || server != nil {
				t.Fatal("unavailable builder admitted")
			}
		})
	}
	var routes []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	routes = append(routes, (commercialResourcesModule{}).routes()...)
	validate := func(r []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{Resources: true})
	}
	if err := validate(routes); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*httproute.Descriptor){func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyPublic }, func(r *httproute.Descriptor) { r.Permission = "" }, func(r *httproute.Descriptor) {
		r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
	}, func(r *httproute.Descriptor) { r.RequestTimeout = 0 }, func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = false }, func(r *httproute.Descriptor) { r.Module = "other" }, func(r *httproute.Descriptor) { r.Handler = nil }} {
		copy := append([]httproute.Descriptor(nil), routes...)
		change(&copy[len(copy)-1])
		if validate(copy) == nil {
			t.Fatal("resource route drift admitted")
		}
	}
	if validate(routes[:len(routes)-1]) == nil {
		t.Fatal("missing resource route admitted")
	}
}
