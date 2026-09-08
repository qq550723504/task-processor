package httpapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/workbenchcontext"
)

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
