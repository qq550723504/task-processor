package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/sourceaccountregistry"
	sourceaccounthttpapi "task-processor/internal/sourceaccountregistry/httpapi"
	"task-processor/internal/workbenchcontext"
)

func TestMountedSourceAccountReportsIdentityExpiringAfterResolutionAsAuthenticationRequired(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	expiresAt := now.Add(time.Minute)
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := sourceaccountregistry.NewService(applicationNoopStore{}, authorizer, sourceaccountregistry.WithClock(func() time.Time {
		return expiresAt.Add(time.Nanosecond)
	}))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := sourceaccounthttpapi.NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	registry := kernelmodule.NewRegistry()
	if err := sourceaccounthttpapi.NewModule(handler).Register(registry); err != nil {
		t.Fatal(err)
	}
	verifier := applicationExpiringVerifier{expiresAt: expiresAt}
	grants := applicationOperatorGrantLoader{}
	resolver := workbenchcontext.NewResolver(grants, "project", "v1", nil, workbenchcontext.WithResolverClock(func() time.Time { return now }))
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, registry.Routes(), routeAuthDependencies{
		workbenchVerifier: verifier, organizationResolver: resolver, authorizer: authorizer,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/workbench/source-accounts", nil)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("X-Requested-Organization-ID", "org-b")
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":"AUTHENTICATION_REQUIRED"`) {
		t.Fatalf("expired-after-resolution response status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestNewSourceAccountApplicationRequiresExplicitSchemaWithoutDDL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "source-account-app.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	grants := applicationGrantLoader{}
	server, err := NewSourceAccountApplication(db, applicationVerifier{}, workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer)
	if err == nil || server != nil {
		t.Fatalf("NewSourceAccountApplication() = %#v, %v", server, err)
	}
	for _, table := range []string{"source_account_resources", "source_account_operations", "source_account", "organization_source_accounts"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("application construction created %s", table)
		}
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("caller-owned database was not usable after construction failure: %v", err)
	}
}

func TestNewSourceAccountApplicationRejectsNilDatabase(t *testing.T) {
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	grants := applicationGrantLoader{}
	resolver := workbenchcontext.NewResolver(grants, "project", "v1", nil)
	server, err := NewSourceAccountApplication(nil, applicationVerifier{}, resolver, authorizer)
	if err == nil || server != nil {
		t.Fatalf("NewSourceAccountApplication() = %#v, %v", server, err)
	}
}

type applicationVerifier struct{}

func (applicationVerifier) Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: "actor-1", HomeOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)}, nil
}

type applicationGrantLoader struct{}

func (applicationGrantLoader) Load(context.Context, workbenchcontext.GrantSource, workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	return workbenchcontext.GrantResult{}, nil
}

func (applicationGrantLoader) Invalidate(string, string) {}

type applicationExpiringVerifier struct{ expiresAt time.Time }

func (verifier applicationExpiringVerifier) Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
	return authidentity.AuthenticatedIdentity{UserID: "actor-1", HomeOrganizationID: "org-a", TokenExpiresAt: verifier.expiresAt}, nil
}

type applicationOperatorGrantLoader struct{}

func (applicationOperatorGrantLoader) Load(_ context.Context, source workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	return workbenchcontext.GrantResult{Source: source, Grants: []authidentity.OrganizationGrant{{
		OrganizationID: "org-b", ProjectID: "project", Roles: []string{"listingkit_operator"},
	}}}, nil
}

func (applicationOperatorGrantLoader) Invalidate(string, string) {}

type applicationNoopStore struct{}

func (applicationNoopStore) Run(context.Context, sourceaccountregistry.Operation, func(sourceaccountregistry.Transaction) (sourceaccountregistry.Account, error)) (sourceaccountregistry.MutationResult, error) {
	return sourceaccountregistry.MutationResult{}, sourceaccountregistry.ErrUnavailable
}

func (applicationNoopStore) Read(context.Context, sourceaccountregistry.Scope, string) (sourceaccountregistry.Account, error) {
	return sourceaccountregistry.Account{}, sourceaccountregistry.ErrUnavailable
}

func (applicationNoopStore) List(context.Context, sourceaccountregistry.Scope, sourceaccountregistry.PageRequest) (sourceaccountregistry.Page, error) {
	return sourceaccountregistry.Page{}, sourceaccountregistry.ErrUnavailable
}
