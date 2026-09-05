package commercetoolauth

import (
	"context"
	"testing"

	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
)

func TestNewCasbinAuthorizerRejectsNilDelegate(t *testing.T) {
	if _, err := NewCasbinAuthorizer(nil); err == nil {
		t.Fatal("NewCasbinAuthorizer(nil) error = nil")
	}
}

func TestCasbinAuthorizerDelegatesExistingListingKitPolicy(t *testing.T) {
	delegate, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatalf("NewListingKitAuthorizer(): %v", err)
	}
	authorizer, err := NewCasbinAuthorizer(delegate)
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer(): %v", err)
	}
	requirement := commercetool.PermissionRequirement{Permission: authz.PermissionListingKitAdminRead}

	tests := []struct {
		role    string
		allowed bool
	}{
		{role: "listingkit_operator", allowed: true},
		{role: "listingkit_admin", allowed: true},
		{role: "platform_admin", allowed: true},
		{role: "admin", allowed: true},
		{role: "listingkit_viewer"},
		{role: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			err := authorizer.Authorize(context.Background(), commercetool.Principal{
				TenantID: "tenant-1",
				UserID:   "user-1",
				Roles:    []string{tt.role},
			}, requirement)
			if tt.allowed && err != nil {
				t.Fatalf("Authorize() error = %v", err)
			}
			if !tt.allowed && err == nil {
				t.Fatal("Authorize() error = nil")
			}
		})
	}
}
