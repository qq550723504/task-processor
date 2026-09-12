package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/app/accountaudit"

	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestCurrentApplicationAuditFactoryAdmission(t *testing.T) {
	if defaultCurrentApplicationFactories(context.Background()).buildAccountAudit == nil {
		t.Fatal("default application omitted audit factory")
	}
	for _, mode := range []string{"enabled", "error", "nil", "missing-route", "wrong-permission"} {
		t.Run(mode, func(t *testing.T) {
			cfg := currentApplicationTestConfig()
			sourceDB, commercialDB := &gorm.DB{}, &gorm.DB{}
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					dependencies := newRouteAuthDependencies()
					return workbenchContextBuildResult{module: currentApplicationTestModule{name: "workbench", routes: currentWorkbenchApplicationRoutes[:4]}, authDependencies: &dependencies}, nil
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
					return currentApplicationTestModule{name: "source", routes: currentWorkbenchApplicationRoutes[5:]}, nil
				},
				buildCommercial: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
					return currentApplicationTestModule{name: "commercial", routes: currentWorkbenchApplicationRoutes[4:5]}, nil
				},
				buildAccountAudit: func(got *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
					if got != sourceDB || authorizer == nil {
						t.Fatal("audit did not reuse source pool/authorizer")
					}
					if mode == "error" {
						return nil, errors.New("audit construction failed")
					}
					if mode == "nil" {
						return nil, nil
					}
					query, _ := accountaudit.New(&auditHTTPHistory{})
					return currentAuditTestModule{inner: accountAuditModule{query: query}, mode: mode}, nil
				},
			}
			server, err := buildCurrentApplication(context.Background(), sourceDB, commercialDB, cfg, logrus.New(), factories)
			if mode == "enabled" {
				if err != nil || server == nil {
					t.Fatalf("audit assembly: %v", err)
				}
			} else if err == nil || server != nil {
				t.Fatalf("%s did not fail closed: %v", mode, err)
			}
		})
	}
}

type currentAuditTestModule struct {
	inner accountAuditModule
	mode  string
}

func (m currentAuditTestModule) Name() string                { return "account-audit" }
func (m currentAuditTestModule) Enabled(*config.Config) bool { return true }
func (m currentAuditTestModule) Register(registry *kernelmodule.Registry) error {
	if m.mode == "missing-route" {
		return nil
	}
	other := kernelmodule.NewRegistry()
	if err := m.inner.Register(other); err != nil {
		return err
	}
	routes := other.Routes()
	if m.mode == "wrong-permission" {
		routes[0].Permission = authz.PermissionWorkbenchSourceAccountManage
	}
	registry.AddRoutes(routes...)
	return nil
}

func TestCurrentApplicationAuditRequiresExactDescriptor(t *testing.T) {
	query, _ := accountaudit.New(&auditHTTPHistory{})
	audit := kernelmodule.NewRegistry()
	_ = (accountAuditModule{query: query}).Register(audit)
	base := make([]httproute.Descriptor, 0, 11)
	for _, route := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	valid := append(append([]httproute.Descriptor{}, base...), audit.Routes()[0])
	if err := validateCurrentApplicationRoutes(valid, true); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*httproute.Descriptor){func(r *httproute.Descriptor) { r.Method = "POST" }, func(r *httproute.Descriptor) { r.Path += "/extra" }, func(r *httproute.Descriptor) {
		r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
	}, func(r *httproute.Descriptor) { r.Permission = "" }, func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyPublic }, func(r *httproute.Descriptor) { r.RequestTimeout = 0 }, func(r *httproute.Descriptor) { r.OrganizationTargetResolver = nil }, func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = false }} {
		routes := append([]httproute.Descriptor{}, valid...)
		mutate(&routes[len(routes)-1])
		if err := validateCurrentApplicationRoutes(routes, true); err == nil {
			t.Fatal("invalid audit descriptor allowed")
		}
	}
	for _, routes := range [][]httproute.Descriptor{base, append(valid, valid[len(valid)-1]), append(append([]httproute.Descriptor{}, valid[:10]...), valid[0])} {
		if err := validateCurrentApplicationRoutes(routes, true); err == nil {
			t.Fatal("invalid route set allowed")
		}
	}
	if err := validateCurrentApplicationRoutes(valid, false); err == nil {
		t.Fatal("unrequested audit allowed")
	}
}

func TestBuildCurrentApplicationAssemblesOnlyTenAdmittedRoutes(t *testing.T) {
	sourceDB, commercialDB := &gorm.DB{}, &gorm.DB{}
	cfg := currentApplicationTestConfig()
	called := map[string]bool{}
	factories := currentApplicationFactories{
		buildWorkbench: func(got *config.Config, _ *logrus.Logger) (workbenchContextBuildResult, error) {
			if got != cfg {
				t.Fatal("workbench builder received another config")
			}
			called["workbench"] = true
			dependencies := newRouteAuthDependencies()
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "workbench", routes: currentWorkbenchApplicationRoutes[:4]}, authDependencies: &dependencies}, nil
		},
		buildSourceAccount: func(got *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			if got != sourceDB || authorizer == nil {
				t.Fatal("source account builder did not receive its owned database and authorizer")
			}
			called["source"] = true
			return currentApplicationTestModule{name: "source", routes: currentWorkbenchApplicationRoutes[5:]}, nil
		},
		buildCommercial: func(got *gorm.DB, authorizer *authz.ListingKitAuthorizer) (kernelmodule.Module, error) {
			if got != commercialDB || authorizer == nil {
				t.Fatal("commercial builder did not receive its read database and authorizer")
			}
			called["commercial"] = true
			return currentApplicationTestModule{name: "commercial", routes: currentWorkbenchApplicationRoutes[4:5]}, nil
		},
	}

	server, err := buildCurrentApplication(context.Background(), sourceDB, commercialDB, cfg, logrus.New(), factories)
	if err != nil {
		t.Fatalf("buildCurrentApplication() error = %v", err)
	}
	if server.Addr != "127.0.0.1:0" || server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("server transport = %#v", server)
	}
	for _, dependency := range []string{"workbench", "source", "commercial"} {
		if !called[dependency] {
			t.Fatalf("%s builder was not called", dependency)
		}
	}
	for _, route := range currentWorkbenchApplicationRoutes {
		path := route.Path
		if path == "/api/v1/workbench/source-accounts/:source_account_id" {
			path = "/api/v1/workbench/source-accounts/01991e24-61ab-7f5f-85d1-3157bb8b75c1"
		}
		if path == "/api/v1/workbench/source-accounts/:source_account_id/disable" {
			path = "/api/v1/workbench/source-accounts/01991e24-61ab-7f5f-85d1-3157bb8b75c1/disable"
		}
		if path == "/api/v1/workbench/source-accounts/:source_account_id/enable" {
			path = "/api/v1/workbench/source-accounts/01991e24-61ab-7f5f-85d1-3157bb8b75c1/enable"
		}
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, httptest.NewRequest(route.Method, path, nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s %s status = %d", route.Method, path, response.Code)
		}
	}
	missing := httptest.NewRecorder()
	server.Handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/legacy", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("legacy route status = %d", missing.Code)
	}
}

func TestBuildCurrentApplicationRejectsRouteDrift(t *testing.T) {
	dependencies := newRouteAuthDependencies()
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "drift", routes: []currentApplicationRoute{{Method: http.MethodGet, Path: "/api/v1/legacy"}}}, authDependencies: &dependencies}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
		buildCommercial:    func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
	}

	server, err := buildCurrentApplication(context.Background(), &gorm.DB{}, &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
	if err == nil || server != nil {
		t.Fatalf("route drift build = %#v, %v", server, err)
	}
}

func currentApplicationTestConfig() *config.Config {
	return &config.Config{Workbench: config.WorkbenchConfig{Enabled: true}, ListingKit: config.ListingKitConfig{Zitadel: config.ListingKitZitadelConfig{
		IssuerURL: "http://localhost:18080", AuthorizationAPIURL: "http://localhost:18080", ClientID: "client", ClientSecret: "secret", ProjectID: "project", AuthorizationRequired: true,
	}}}
}

type currentApplicationTestModule struct {
	name   string
	routes []currentApplicationRoute
}

func (module currentApplicationTestModule) Name() string         { return module.name }
func (currentApplicationTestModule) Enabled(*config.Config) bool { return true }
func (module currentApplicationTestModule) Register(reg *kernelmodule.Registry) error {
	for _, route := range module.routes {
		reg.AddRoutes(httproute.Descriptor{Method: route.Method, Path: route.Path, AuthPolicy: httproute.AuthPolicyPublic, Handler: func(c *gin.Context) { c.Status(http.StatusNoContent) }})
	}
	return nil
}

func TestCurrentWorkbenchApplicationRouteContractIsStable(t *testing.T) {
	got := make([]string, 0, len(currentWorkbenchApplicationRoutes))
	for _, route := range currentWorkbenchApplicationRoutes {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)
	if len(got) != 10 {
		t.Fatalf("route contract contains %d routes: %v", len(got), got)
	}
}
