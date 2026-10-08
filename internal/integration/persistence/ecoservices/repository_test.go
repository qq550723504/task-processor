package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	e "task-processor/internal/ecoservices"
	"testing"
)

func fixture(t *testing.T) (*Repository, *e.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	service, err := e.NewService(repo, nil, 180)
	if err != nil {
		t.Fatal(err)
	}
	return repo, service
}
func TestRequestPersistenceIdempotencyAndCrossOrganization(t *testing.T) {
	repo, service := fixture(t)
	ctx := context.Background()
	listingID := uuid.NewString()
	listing := e.Listing{ID: listingID, ProviderOrganizationID: "provider", Title: "公司注册", Category: e.CompanyRegistration, State: "PUBLISHED", Version: 1}
	if err := repo.db.Create(listingRecord(listing)).Error; err != nil {
		t.Fatal(err)
	}
	c := e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "b"}, Key: uuid.NewString(), Kind: "request_create", ID: listingID, Description: "上海公司注册"}
	first, err := service.Mutate(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Mutate(ctx, c)
	if err != nil || again.Request.ID != first.Request.ID {
		t.Fatalf("duplicate request: %+v %v", again, err)
	}
	c.Description = "different"
	if _, err := service.Mutate(ctx, c); !errors.Is(err, e.ErrConflict) {
		t.Fatalf("same-key changed intent accepted: %v", err)
	}
	for _, org := range []string{"buyer", "provider", "stranger"} {
		q := e.Query{Scope: e.Scope{OrganizationID: org, ActorID: org}, Kind: "requests", ID: first.Request.ID, Page: 1, PageSize: 20}
		page, err := service.Read(ctx, q)
		if org == "stranger" {
			if !errors.Is(err, e.ErrNotFound) {
				t.Fatalf("foreign request readable: %+v %v", page, err)
			}
		} else if err != nil || len(page.Requests) != 1 {
			t.Fatalf("bound party cannot read: %s %+v %v", org, page, err)
		}
	}
}
func TestUnqualifiedProviderCannotPublish(t *testing.T) {
	_, service := fixture(t)
	c := e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "p"}, Key: uuid.NewString(), Kind: "listing_create", Listing: &e.Listing{Title: "注册公司", Description: "注册服务", Category: e.CompanyRegistration, Items: []string{"企业登记"}, Regions: []string{"上海"}, PriceMinor: 10000, DeliveryDays: 7}}
	if _, err := service.Mutate(context.Background(), c); !errors.Is(err, e.ErrNotQualified) {
		t.Fatalf("unqualified provider can publish service: %v", err)
	}
}
