package httpapi

import (
	"context"
	"task-processor/internal/authidentity"
	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"testing"
	"time"
)

func TestTopUpAuthorizationSeparatesPlatformRefundAndTenantPayment(t *testing.T) {
	reader := &exactAuthorizationReaderStub{result: zitadelruntime.ExactServiceProjectAuthorization{Found: true, State: "STATE_ACTIVE", Roles: []string{"listingkit_admin"}}}
	a := topUpRuntimeAuthorizer{directory: financialRecoveryAuthorizer{reader: reader, serviceToken: "fixture-service-token", projectID: "project-1", authorizer: authz.DefaultListingKitAuthorizer()}}
	ctx := func(i authidentity.AuthenticatedIdentity) context.Context {
		return authidentity.WithAuthenticatedIdentity(context.Background(), i)
	}
	tenant := authidentity.AuthenticatedIdentity{UserID: "actor-1", TenantID: "org-1", EffectiveOrganizationID: "org-1", Roles: []string{"listingkit_admin"}}
	if err := a.AuthorizeTopUp(ctx(tenant), "org-1", "actor-1", false); err != nil {
		t.Fatal(err)
	}
	reader.result.State = "STATE_INACTIVE"
	if a.AuthorizeTopUp(ctx(tenant), "org-1", "actor-1", false) == nil {
		t.Fatal("revoked member admitted")
	}
	if a.AuthorizeTopUp(ctx(tenant), "", "actor-1", true) == nil {
		t.Fatal("tenant administrator refunded")
	}
	admin := authidentity.AuthenticatedIdentity{UserID: "platform-1", Roles: []string{"platform_admin"}}
	reader.organization = ""
	if err := a.AuthorizeTopUp(ctx(admin), "", "platform-1", true); err != nil {
		t.Fatal(err)
	}
	if reader.organization != "" {
		t.Fatal("platform refund required beneficiary membership")
	}
	if a.AuthorizeTopUp(context.Background(), "", "platform-1", true) == nil {
		t.Fatal("stored actor alone authorized refund")
	}
	admin.TokenExpiresAt = time.Now().Add(-time.Minute)
	if a.AuthorizeTopUp(ctx(admin), "", "platform-1", true) == nil {
		t.Fatal("expired authority admitted")
	}
}
