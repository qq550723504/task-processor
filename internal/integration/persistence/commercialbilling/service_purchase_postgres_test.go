//go:build integration

package commercialbilling

import (
	"context"
	"errors"
	"github.com/google/uuid"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"sync"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestServicePurchasePostgresDispatchLeaseAndDurableFacts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("billing"), pg.WithUsername("service_installer"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	open := func(dsn string) *gorm.DB {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal("fixture connection failed")
		}
		pool, _ := db.DB()
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	owner := open(dsn)
	if err := AutoMigrate(owner); err != nil {
		t.Fatal(err)
	}
	password := uuid.NewString()
	if err := owner.Exec(`CREATE ROLE commercial_owner_runtime LOGIN PASSWORD '` + password + `'; REVOKE CREATE,TEMP ON DATABASE billing FROM PUBLIC; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO commercial_owner_runtime;
 GRANT SELECT,INSERT,UPDATE ON commercial_service_orders TO commercial_owner_runtime;
 GRANT SELECT,INSERT ON commercial_service_operations,commercial_service_payment_inbox,commercial_service_effect_inbox TO commercial_owner_runtime;`).Error; err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(dsn)
	endpoint.User = url.UserPassword("commercial_owner_runtime", password)
	db := open(endpoint.String())
	if err := VerifyServicePurchasesRuntime(ctx, db); err != nil {
		t.Fatal(err)
	}
	r, _ := New(db)
	source := billing.ServicePurchaseCommand{ID: "create-proof", RequestID: "request", OrderID: "service-order", Kind: "CREATE_PURCHASE", SourceProofID: "create-proof", ActorID: "buyer", BuyerOrganizationID: "buyer-org", ProviderOrganizationID: "provider-org", ProviderMerchantID: "sub-merchant", QuoteVersion: 1, AmountMinor: 100, DeliveryDays: 7, PolicyVersion: "10-percent-platform-fee"}
	profile := billing.ServiceMerchantProfile{Version: "profile", Environment: "PRODUCTION", PlatformMerchantID: "platform", AppID: "app", FreezeDays: 180}
	var wg sync.WaitGroup
	orders := make(chan billing.ServicePurchaseOrder, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			o, err := r.CreateServicePurchase(ctx, source, profile)
			if err != nil {
				failures <- err
			} else {
				orders <- o
			}
		})
	}
	wg.Wait()
	close(orders)
	close(failures)
	if len(orders) != 8 || len(failures) > 0 {
		for err := range failures {
			t.Error(err)
		}
		t.Fatal("original order creation did not converge")
	}
	var original billing.ServicePurchaseOrder
	for o := range orders {
		if original.Source.OrderID != "" && original.Fingerprint() != o.Fingerprint() {
			t.Fatal("multiple original payment identities")
		}
		original = o
	}
	claims := make(chan billing.ServicePurchaseOrder, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			o, err := r.ClaimServicePurchase(ctx, source.OrderID, uuid.NewString(), time.Now().Add(time.Minute))
			if err == nil {
				claims <- o
			}
		})
	}
	wg.Wait()
	close(claims)
	if len(claims) != 1 {
		t.Fatalf("multiple dispatch owners: %d", len(claims))
	}
	old := <-claims
	payment := billing.ServicePaymentObservation{EventID: "verified-paid", ProfileVersion: profile.Version, PlatformMerchantID: profile.PlatformMerchantID, AppID: profile.AppID, ProviderMerchantID: source.ProviderMerchantID, TradeNo: old.TradeNo, TransactionID: "original-channel-tx", Currency: "CNY", State: "PAID", VerificationVersion: "controlled-verified-fixture", AmountMinor: 100, OccurredAt: time.Now().UTC().Truncate(time.Microsecond)}
	old.Payment = &payment
	old.Operation = &billing.ServiceFinancialOperation{Reservation: money.ServiceOperation{OrderID: source.OrderID, OperationID: "original-share-operation", Kind: money.ServiceShare, AmountMinor: 10, SourceProofID: "accepted-delivery"}, CommandID: "accepted", ProviderRequestID: "00000000000000000000000000000001", Dispatched: true}
	old, err = r.SaveServicePurchase(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(65 * time.Second)
	r.now = func() time.Time { return later }
	newOwner, err := r.ClaimServicePurchase(ctx, source.OrderID, "new-owner", later.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	old.State = "SETTLED"
	if _, err := r.SaveServicePurchase(ctx, old); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("expired worker overwrote new owner: %v", err)
	}
	effect := billing.ServiceOperationObservation{EventID: "late-original-share-success", ProfileVersion: profile.Version, ProviderMerchantID: source.ProviderMerchantID, TransactionID: payment.TransactionID, ProviderRequestID: old.Operation.ProviderRequestID, Kind: money.ServiceShare, AmountMinor: 10, State: "SUCCESS", ProviderReference: "original-share-detail", VerificationVersion: "controlled-verified-fixture", OccurredAt: payment.OccurredAt.Add(time.Minute)}
	if err := r.RecordServiceOperationObservation(ctx, old, *old.Operation, effect); err != nil {
		t.Fatalf("expired worker lost signed original fact %v", err)
	}
	facts, err := r.ServiceOperationObservations(ctx, newOwner.Operation.Reservation.OperationID)
	if err != nil || len(facts) != 1 || facts[0].ProviderReference != effect.ProviderReference {
		t.Fatalf("new owner cannot recover original proof %+v %v", facts, err)
	}
	if err := owner.Exec("GRANT UPDATE(payload) ON commercial_service_effect_inbox TO commercial_owner_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if err := VerifyServicePurchasesRuntime(ctx, db); !errors.Is(err, billing.ErrFeatureUnavailable) {
		t.Fatalf("mutable signed fact accepted: %v", err)
	}
}
