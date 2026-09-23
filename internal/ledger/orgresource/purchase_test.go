package orgresource

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type purchasedGrantAuthorizerStub struct {
	max int64
	err error
}

func (stub purchasedGrantAuthorizerStub) AuthorizePurchasedGrant(context.Context, Principal, ResourceType) (PurchasedGrantAuthorization, error) {
	if stub.err != nil {
		return PurchasedGrantAuthorization{}, stub.err
	}
	return PurchasedGrantAuthorization{MaxQuantity: stub.max}, nil
}

type purchasedGrantExecutorStub struct {
	executed PurchasedResourceGrantExecution
	replay   PurchasedResourceGrantResult
	found    bool
	err      error
}

func (stub *purchasedGrantExecutorStub) ReplayPurchasedResourceGrant(context.Context, PurchasedResourceGrantReplay) (PurchasedResourceGrantResult, bool, error) {
	return stub.replay, stub.found, stub.err
}

func (stub *purchasedGrantExecutorStub) ExecutePurchasedResourceGrant(_ context.Context, input PurchasedResourceGrantExecution) (PurchasedResourceGrantResult, error) {
	stub.executed = input
	return PurchasedResourceGrantResult{Snapshot: PurchasedResourceGrantSnapshot{
		OperationID:           input.OperationID,
		OrganizationID:        input.OrganizationID,
		CommercialOrderID:     input.CommercialOrderID,
		CommercialOrderItemID: input.CommercialOrderItemID,
		ResourceType:          input.ResourceType,
		Quantity:              "10",
		SourceType:            input.SourceType,
		SourceIdentity:        input.SourceIdentity,
	}}, nil
}

func TestPurchasedResourceGrantFixesCommercialSourceIdentity(t *testing.T) {
	executor := &purchasedGrantExecutorStub{}
	service, err := NewPurchasedResourceGrantService(executor, purchasedGrantAuthorizerStub{max: 100})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.GrantPurchasedResource(context.Background(), PurchasedResourceGrantInput{
		OrganizationID:        "org-1",
		OperationID:           "operation-1",
		CommercialOrderID:     "order-1",
		CommercialOrderItemID: "item-1",
		ResourceType:          ResourceAIPoint,
		Quantity:              10,
		Principal:             Principal{ID: "commercial-owner", Kind: PrincipalTrustedCommercial},
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	if executor.executed.SourceType != SourceCommercialOrderItem {
		t.Fatalf("source type = %q", executor.executed.SourceType)
	}
	wantSourceIdentity := purchasedGrantSourceIdentity("org-1", "order-1", "item-1")
	if executor.executed.SourceIdentity != wantSourceIdentity {
		t.Fatalf("source identity = %q", executor.executed.SourceIdentity)
	}
	if executor.executed.RequestFingerprint == "" {
		t.Fatalf("request fingerprint must be fixed by resource owner")
	}
	if result.Snapshot.SourceIdentity != wantSourceIdentity {
		t.Fatalf("snapshot source identity = %q", result.Snapshot.SourceIdentity)
	}
}

func TestPurchasedResourceGrantRejectsQuantityBeyondRegisteredCommercialLimit(t *testing.T) {
	executor := &purchasedGrantExecutorStub{}
	service, err := NewPurchasedResourceGrantService(executor, purchasedGrantAuthorizerStub{max: 5})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = service.GrantPurchasedResource(context.Background(), PurchasedResourceGrantInput{
		OrganizationID:        "org-1",
		OperationID:           "operation-1",
		CommercialOrderID:     "order-1",
		CommercialOrderItemID: "item-1",
		ResourceType:          ResourceDataRow,
		Quantity:              6,
		Principal:             Principal{ID: "commercial-owner", Kind: PrincipalTrustedCommercial},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if executor.executed.OperationID != "" {
		t.Fatalf("executor must not run after quantity rejection")
	}
}

func TestPurchasedResourceGrantReplaysBeforeApplyingCurrentQuantityLimit(t *testing.T) {
	executor := &purchasedGrantExecutorStub{
		found: true,
		replay: PurchasedResourceGrantResult{Snapshot: PurchasedResourceGrantSnapshot{
			OperationID: "operation-1",
			Quantity:    "10",
		}},
	}
	service, err := NewPurchasedResourceGrantService(executor, purchasedGrantAuthorizerStub{max: 5})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.GrantPurchasedResource(context.Background(), PurchasedResourceGrantInput{
		OrganizationID:        "org-1",
		OperationID:           "operation-1",
		CommercialOrderID:     "order-1",
		CommercialOrderItemID: "item-1",
		ResourceType:          ResourceDataRow,
		Quantity:              10,
		Principal:             Principal{ID: "commercial-owner", Kind: PrincipalTrustedCommercial},
	})
	if err != nil {
		t.Fatalf("replay must not be rejected by a changed quantity limit: %v", err)
	}
	if !result.Replayed || executor.executed.OperationID != "" {
		t.Fatalf("result = %#v, execution = %#v", result, executor.executed)
	}
}

func TestPurchasedResourceGrantSourceIdentityIsUnambiguous(t *testing.T) {
	if first, second := purchasedGrantSourceIdentity("org", "order:a", "item"), purchasedGrantSourceIdentity("org", "order", "a:item"); first == second {
		t.Fatalf("source identities collide: %q", first)
	}
}

func TestPurchasedResourceGrantSourceIdentityFitsPersistenceLimit(t *testing.T) {
	identity := purchasedGrantSourceIdentity(strings.Repeat("g", 128), strings.Repeat("o", 128), strings.Repeat("i", 128))
	if len(identity) > 192 {
		t.Fatalf("source identity length = %d, want <= 192", len(identity))
	}
}

func TestPurchasedResourceGrantSourceIdentityIncludesOrganization(t *testing.T) {
	firstExecutor := &purchasedGrantExecutorStub{}
	firstService, err := NewPurchasedResourceGrantService(firstExecutor, purchasedGrantAuthorizerStub{max: 100})
	if err != nil {
		t.Fatalf("new first service: %v", err)
	}
	secondExecutor := &purchasedGrantExecutorStub{}
	secondService, err := NewPurchasedResourceGrantService(secondExecutor, purchasedGrantAuthorizerStub{max: 100})
	if err != nil {
		t.Fatalf("new second service: %v", err)
	}
	base := PurchasedResourceGrantInput{
		OperationID:           "operation-1",
		CommercialOrderID:     "order-1",
		CommercialOrderItemID: "item-1",
		ResourceType:          ResourceAIPoint,
		Quantity:              1,
		Principal:             Principal{ID: "commercial-owner", Kind: PrincipalTrustedCommercial},
	}
	first := base
	first.OrganizationID = "org-1"
	second := base
	second.OrganizationID = "org-2"
	if _, err := firstService.GrantPurchasedResource(context.Background(), first); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	if _, err := secondService.GrantPurchasedResource(context.Background(), second); err != nil {
		t.Fatalf("second grant: %v", err)
	}
	if firstExecutor.executed.SourceIdentity == secondExecutor.executed.SourceIdentity {
		t.Fatalf("source identity is shared across organizations: %q", firstExecutor.executed.SourceIdentity)
	}
}

func TestPurchasedResourceGrantRequiresTrustedAuthorizer(t *testing.T) {
	executor := &purchasedGrantExecutorStub{}
	service, err := NewPurchasedResourceGrantService(executor, purchasedGrantAuthorizerStub{max: 100, err: errors.New("denied")})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	_, err = service.GrantPurchasedResource(context.Background(), PurchasedResourceGrantInput{
		OrganizationID:        "org-1",
		OperationID:           "operation-1",
		CommercialOrderID:     "order-1",
		CommercialOrderItemID: "item-1",
		ResourceType:          ResourceStoreRenewalPeriod,
		Quantity:              1,
		Principal:             Principal{ID: "browser-user", Kind: PrincipalTenantHuman},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
