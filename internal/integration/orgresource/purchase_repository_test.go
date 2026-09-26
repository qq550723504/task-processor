package orgresourceadapter

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/ledger/orgresource"
)

func TestPurchasedResourceGrantIsSourceBoundAndReplayable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:commercial-purchase-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := NewGormRepository(db, TransactionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := orgresource.NewPurchasedResourceGrantService(repository, TrustedCommercialGrantAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	input := orgresource.PurchasedResourceGrantInput{OrganizationID: "org-a", OperationID: "grant:order-1", CommercialOrderID: "order-1", CommercialOrderItemID: "item-1", ResourceType: orgresource.ResourceAIPoint, Quantity: 10, Principal: orgresource.Principal{ID: "commercial-billing", Kind: orgresource.PrincipalTrustedCommercial}}
	first, err := service.GrantPurchasedResource(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.GrantPurchasedResource(context.Background(), input)
	if err != nil || !second.Replayed || second.Snapshot.EventID != first.Snapshot.EventID {
		t.Fatalf("replay=%#v err=%v first=%#v", second, err, first)
	}
	wallet := organizationResourceBucketRow{}
	if err := db.Where("organization_id = ? AND resource_type = ?", "org-a", orgresource.ResourceAIPoint).Take(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.Available != 10 {
		t.Fatalf("resource wallet=%#v", wallet)
	}
}
