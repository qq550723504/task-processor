package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	e "task-processor/internal/ecoservices"
	"testing"
	"time"
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
func TestCatalogGroupCountsAndDueOrdersAreOwnerFacts(t *testing.T) {
	r, service := fixture(t)
	ctx := context.Background()
	scope := e.Scope{OrganizationID: "buyer", ActorID: "buyer-user"}
	if err := r.db.Create(applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", Version: 1, MerchantID: "fixture-original-sub"})).Error; err != nil {
		t.Fatal(err)
	}
	for _, category := range []e.Category{e.CompanyRegistration, e.TrademarkRegistration, e.StoreOpening} {
		if err := r.db.Create(listingRecord(e.Listing{ID: uuid.NewString(), ProviderOrganizationID: "provider", Category: category, Title: string(category), State: "PUBLISHED", Version: 1})).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.Read(ctx, e.Query{Scope: scope, Kind: "catalog", Group: "enterprise", Page: 1, PageSize: 1})
	if err != nil || page.Total != 2 || len(page.Listings) != 1 || page.Counts[string(e.StoreOpening)] != 1 {
		t.Fatalf("group/page counts came from visible slice: %+v %v", page, err)
	}
	for _, days := range []int{2, 12} {
		expiry := time.Now().UTC().AddDate(0, 0, days)
		if err := r.db.Create(requestRecord(e.Request{ID: uuid.NewString(), BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "AWAITING_ACCEPTANCE", Version: 1, FundsExpireAt: &expiry, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err = service.Read(ctx, e.Query{Scope: e.Scope{Platform: true, ActorID: "platform-user"}, Kind: "due_orders", Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Requests) != 1 || page.Requests[0].AcceptanceID != "" || page.Requests[0].State != "AWAITING_ACCEPTANCE" {
		t.Fatalf("manual due queue ignored actual expiry or fabricated acceptance: %+v %v", page, err)
	}
}
func TestRequestPersistenceIdempotencyAndCrossOrganization(t *testing.T) {
	repo, service := fixture(t)
	ctx := context.Background()
	if err := repo.db.Create(applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", Version: 1, MerchantID: "fixture-original-sub"})).Error; err != nil {
		t.Fatal(err)
	}
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
func TestVerifiedPaymentWakeRetainsOriginalCommandAndDrainsDuplicate(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	requestID, orderID := uuid.NewString(), uuid.NewString()
	c := e.FinancialCommand{ID: uuid.NewString(), RequestID: requestID, OrderID: orderID, Kind: "CREATE_PURCHASE", SourceProofID: uuid.NewString(), ActorID: "buyer", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "PENDING"}
	raw, _ := json.Marshal(c)
	if err := r.db.Create(&financialRow{ID: c.ID, RequestID: requestID, OrderID: orderID, Kind: c.Kind, Fingerprint: e.Fingerprint(c), Payload: raw, State: "DONE"}).Error; err != nil {
		t.Fatal(err)
	}
	req := e.Request{ID: requestID, OrderID: orderID, State: "PAID_READY", Version: 2, PaymentReceiptID: "original-paid", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider"}
	if err := r.db.Create(requestRecord(req)).Error; err != nil {
		t.Fatal(err)
	}
	result := e.FinancialResult{OrderID: orderID, PaymentReceiptID: "original-paid", State: "PAID", Revision: 4}
	data, _ := json.Marshal(result)
	if err := r.db.Model(&financialRow{}).Where("id=?", c.ID).Update("result", data).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.WakeOriginalServicePurchase(ctx, orderID); err != nil {
		t.Fatal(err)
	}
	commands, err := r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 {
		t.Fatalf("missing original wake %v", err)
	}
	c = commands[0]
	// A second verified inbox write arrives after this worker selected the row.
	if err := r.WakeOriginalServicePurchase(ctx, orderID); err != nil {
		t.Fatal(err)
	}
	if err := r.CompleteFinancialCommand(ctx, c, result); err != nil {
		t.Fatal(err)
	}
	var row financialRow
	r.db.Where("id=?", c.ID).Take(&row)
	if row.State != "PROCESSING" {
		t.Fatal("old worker erased a newer verified notification wake")
	}
	commands, err = r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 {
		t.Fatalf("newer wake not recoverable: %v", err)
	}
	c = commands[0]
	if err := r.CompleteFinancialCommand(ctx, c, result); err != nil {
		t.Fatal(err)
	}
	r.db.Where("id=?", c.ID).Take(&row)
	if row.State != "DONE" || row.ID != c.ID {
		t.Fatalf("duplicate verified notification did not drain original recovery: state=%s", row.State)
	}
}

func TestExpiredPaidOriginalCommandRemainsObservableWithoutChangingAcceptance(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	id, order, request := uuid.NewString(), uuid.NewString(), uuid.NewString()
	expired := time.Now().UTC().Add(-time.Hour)
	c := e.FinancialCommand{ID: id, OrderID: order, RequestID: request, Kind: "CREATE_PURCHASE", State: "PENDING"}
	raw, _ := json.Marshal(c)
	if err := r.db.Create(&financialRow{ID: id, OrderID: order, RequestID: request, Kind: c.Kind, State: "DONE", Fingerprint: e.Fingerprint(c), Payload: raw}).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Create(requestRecord(e.Request{ID: request, OrderID: order, State: "AWAITING_ACCEPTANCE", PaymentReceiptID: "original-paid", FundsExpireAt: &expired, Version: 4})).Error; err != nil {
		t.Fatal(err)
	}
	commands, err := r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 1 || commands[0].ID != id {
		t.Fatalf("expired original no longer observable: %+v %v", commands, err)
	}
	var row requestRow
	r.db.Where("id=?", request).Take(&row)
	if row.State != "AWAITING_ACCEPTANCE" || row.Version != 4 {
		t.Fatal("clock changed customer acceptance")
	}
	commands, err = r.PendingFinancialCommands(ctx, 20)
	if err != nil || len(commands) != 0 {
		t.Fatal("completed expiry observation became a busy polling loop")
	}
}

type originalPaymentRecovery struct{ paidOrder string }

func (t originalPaymentRecovery) ReadServiceRefundableAmount(context.Context, string) (int64, error) {
	return 0, e.ErrUnavailable
}

func (t originalPaymentRecovery) ExecuteServiceCommand(_ context.Context, c e.FinancialCommand) (e.FinancialResult, error) {
	r := e.FinancialResult{OrderID: c.OrderID, State: "AWAITING_PAYMENT", Revision: 3}
	if c.OrderID == t.paidOrder {
		r.State = "PAID"
		r.PaymentReceiptID = "verified-original-payment"
	}
	return r, nil
}
func TestVerifiedPaymentBehindTwentyWaitingOrdersStillRecovers(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	paid := uuid.NewString()
	var paidRequest string
	for i := 0; i < 21; i++ {
		orderID, requestID := uuid.NewString(), uuid.NewString()
		state := "PROCESSING"
		if i == 20 {
			orderID = paid
			paidRequest = requestID
			state = "DONE"
		}
		c := e.FinancialCommand{ID: uuid.NewString(), RequestID: requestID, OrderID: orderID, Kind: "CREATE_PURCHASE", State: "PENDING"}
		raw, _ := json.Marshal(c)
		if err := r.db.Create(&financialRow{ID: c.ID, RequestID: requestID, OrderID: orderID, Kind: c.Kind, Fingerprint: e.Fingerprint(c), Payload: raw, State: state, DispatchAdmitted: i < 20, CreatedAt: time.Now().UTC().Add(time.Duration(i-21) * time.Minute)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := r.db.Create(requestRecord(e.Request{ID: requestID, OrderID: orderID, State: "ORDER_PENDING", Version: 1, BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider"})).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := r.WakeOriginalServicePurchase(ctx, paid); err != nil {
		t.Fatal(err)
	}
	s, err := e.NewService(r, originalPaymentRecovery{paidOrder: paid}, 180)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Recover(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var row requestRow
	r.db.Where("id=?", paidRequest).Take(&row)
	req, _ := requestFact(row)
	if req.State != "PAID_READY" || req.PaymentReceiptID != "verified-original-payment" {
		t.Fatalf("a paid order starved behind 20 waiting original commands: state=%s receipt=%s", req.State, req.PaymentReceiptID)
	}
}

func TestProviderManagementQualificationIsScopedCanonicalProjection(t *testing.T) {
	r, service := fixture(t)
	ctx := context.Background()
	scope := e.Scope{OrganizationID: "operator-org", ActorID: "operator"}
	read := func(want bool) {
		t.Helper()
		page, err := service.Read(ctx, e.Query{Scope: scope, Kind: "provider_listings", Page: 1, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(page)
		var result map[string]any
		if err != nil || json.Unmarshal(raw, &result) != nil || result["providerQualified"] != want {
			t.Fatal("manage-only qualification absent or wrong", string(raw), err)
		}
		for _, secret := range []string{"applications", "fileIds", "merchantId", "private-registration", "private-company"} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("private join data exposed", secret)
			}
		}
	}
	// Another enterprise's qualified fact cannot qualify an empty original org.
	foreign := e.Application{ID: uuid.NewString(), OrganizationID: "foreign", State: "ACTIVE", Version: 1, MerchantID: "foreign-merchant"}
	if err := r.db.Create(applicationRecord(foreign)).Error; err != nil {
		t.Fatal(err)
	}
	read(false)
	app := e.Application{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, CompanyName: "private-company", RegistrationNumber: "private-registration", State: "APPROVED", Version: 1, MerchantID: "", OnboardingState: "AUDITING"}
	if err := r.db.Create(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	read(false)
	app.State = "ACTIVE"
	r.db.Save(applicationRecord(app))
	read(false)
	app.MerchantID = "original-merchant"
	r.db.Save(applicationRecord(app))
	read(true)
	// Qualification does not depend on any listing existing or its page.
	page, err := service.Read(ctx, e.Query{Scope: scope, Kind: "catalog", Page: 1, PageSize: 20})
	raw, _ := json.Marshal(page)
	if err != nil || strings.Contains(string(raw), "providerQualified") {
		t.Fatal("non-provider projection leaked qualification", string(raw), err)
	}
	app.State = "APPROVED"
	app.OnboardingState = "FROZEN"
	r.db.Save(applicationRecord(app))
	read(false)
	if err := r.db.Migrator().DropTable(&applicationRow{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Read(ctx, e.Query{Scope: scope, Kind: "provider_listings", Page: 1, PageSize: 20}); err == nil {
		t.Fatal("qualification database error fabricated usable page")
	}
}

func TestCurrentProviderQualificationStillGuardsAllListingMutations(t *testing.T) {
	r, s := fixture(t)
	ctx := context.Background()
	scope := e.Scope{OrganizationID: "provider", ActorID: "operator"}
	app := e.Application{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, State: "ACTIVE", Version: 1, MerchantID: "original-merchant"}
	if err := r.db.Create(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	listing := e.Listing{Category: e.CompanyRegistration, Title: "原服务", Description: "原说明", Items: []string{"企业登记"}, Regions: []string{"上海"}, PriceMinor: 100, DeliveryDays: 7}
	original, err := s.Mutate(ctx, e.Command{Scope: scope, Kind: "listing_create", Key: uuid.NewString(), Listing: &listing})
	if err != nil {
		t.Fatal(err)
	}
	app.State = "APPROVED"
	app.OnboardingState = "FROZEN"
	if err := r.db.Save(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"listing_create", "listing_update", "listing_publish"} {
		_, err := s.Mutate(ctx, e.Command{Scope: scope, Kind: kind, Key: uuid.NewString(), ID: original.Listing.ID, Version: original.Listing.Version, Listing: &listing})
		if !errors.Is(err, e.ErrNotQualified) {
			t.Fatal("cached manage qualification bypassed live guard", kind, err)
		}
	}
}
