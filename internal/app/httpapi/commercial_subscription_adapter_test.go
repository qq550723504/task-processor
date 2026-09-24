package httpapi

import (
	"context"
	"testing"

	zitadelruntime "task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
)

type exactAuthorizationReaderStub struct {
	result       zitadelruntime.ExactServiceProjectAuthorization
	token        string
	actor        string
	project      string
	organization string
}

func (stub *exactAuthorizationReaderStub) ReadExactServiceProjectAuthorization(_ context.Context, token, actor, project, organization string) (zitadelruntime.ExactServiceProjectAuthorization, error) {
	stub.token, stub.actor, stub.project, stub.organization = token, actor, project, organization
	return stub.result, nil
}

func TestSubscriptionRecoveryAuthorizerUsesExactLiveAssignment(t *testing.T) {
	reader := &exactAuthorizationReaderStub{result: zitadelruntime.ExactServiceProjectAuthorization{Found: true, State: "STATE_ACTIVE", Roles: []string{"listingkit_admin"}}}
	adapter := subscriptionPurchaseRecoveryAuthorizer{reader: reader, serviceToken: "service-token", projectID: "project-1", authorizer: authz.DefaultListingKitAuthorizer()}
	decision, err := adapter.ReauthorizeCommercialPurchase(context.Background(), "org-1", "actor-1")
	if err != nil || !decision.Allowed {
		t.Fatalf("decision = %+v, err = %v", decision, err)
	}
	if reader.token != "service-token" || reader.actor != "actor-1" || reader.project != "project-1" || reader.organization != "org-1" {
		t.Fatalf("exact query = token %q actor %q project %q organization %q", reader.token, reader.actor, reader.project, reader.organization)
	}
	reader.result.State = "STATE_INACTIVE"
	decision, err = adapter.ReauthorizeCommercialPurchase(context.Background(), "org-1", "actor-1")
	if err != nil || decision.Allowed {
		t.Fatalf("inactive decision = %+v, err = %v", decision, err)
	}
}
