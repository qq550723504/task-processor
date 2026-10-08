package ecoservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
	bstore "task-processor/internal/integration/persistence/commercialbilling"
	mstore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
)

// Only payment query is expected: a fulfillment check cannot dispatch money.
type fulfillmentPaymentProvider struct {
	billing.ServicePurchaseProvider
}

func (fulfillmentPaymentProvider) Profile() billing.ServiceMerchantProfile {
	return billing.ServiceMerchantProfile{Version: "profile", Environment: "PRODUCTION", PlatformMerchantID: "platform", AppID: "app", FreezeDays: 180}
}
func (fulfillmentPaymentProvider) QueryServicePayment(_ context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	return billing.ServicePaymentObservation{EventID: "paid:" + o.TradeNo, ProfileVersion: o.Profile.Version, PlatformMerchantID: o.Profile.PlatformMerchantID, AppID: o.Profile.AppID, ProviderMerchantID: o.Source.ProviderMerchantID, TradeNo: o.TradeNo, Currency: "CNY", State: "PAID", TransactionID: "original-paid-transaction", AmountMinor: o.Source.AmountMinor, VerificationVersion: "verified-fixture", OccurredAt: time.Now().UTC().Add(-time.Hour)}, nil
}

type fulfillmentProtection struct {
	billing.ServicePayloadProtection
}

func fulfillmentFundsFixture(t *testing.T) (*Repository, *e.Service, *mstore.Repository, *billing.ServicePurchases, *bstore.Repository, e.Request, e.FinancialCommand) {
	t.Helper()
	r, _ := fixture(t)
	req, original := checkoutAdmissionFixture(t, r)
	if err := r.db.Create(applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", Version: 1, MerchantID: original.MerchantID, AgreementAccepted: true, AgreementVersion: e.PolicyVersion, OnboardingState: "FINISH"})).Error; err != nil {
		t.Fatal(err)
	}
	admission := original
	admission.DispatchOperationID = "checkout:" + original.ID
	if _, err := r.AdmitFinancialCommand(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	// Separate owner databases; consumers use the real E -> B -> M adapters.
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bstore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := mstore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	b, err := bstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	purchases, err := billing.NewServicePurchases(b, m, fulfillmentPaymentProvider{}, ecoservicesbilling.Source{Repository: r}, fulfillmentProtection{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.NewService(r, ecoservicesbilling.Trading{Purchases: purchases}, 180)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	page, err := r.Read(context.Background(), e.Query{Scope: e.Scope{OrganizationID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
	if err != nil || len(page.Requests) != 1 || page.Requests[0].State != "PAID_READY" {
		t.Fatalf("payment setup: %+v %v", page, err)
	}
	return r, s, m, purchases, b, page.Requests[0], original
}

func TestCanonicalChargebackBlocksCurrentFulfillmentAndProjectsOriginalFence(t *testing.T) {
	for _, kind := range []string{"start", "deliver", "accept", "reject"} {
		t.Run(kind, func(t *testing.T) {
			r, s, m, _, b, req, original := fulfillmentFundsFixture(t)
			ctx := context.Background()
			if kind != "start" {
				result, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "start", ID: req.ID, Version: req.Version})
				if err != nil {
					t.Fatal(err)
				}
				req = *result.Request
			}
			if kind == "accept" || kind == "reject" {
				result, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "deliver", ID: req.ID, Version: req.Version, Delivery: &e.Delivery{Content: "complete"}})
				if err != nil {
					t.Fatal(err)
				}
				req = *result.Request
			}
			order, err := b.ReadServicePurchase(ctx, req.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			cb := money.ChargebackSettlement{ChargebackID: "signed-chargeback", PaymentID: order.MoneyInput().Payment.PaymentID, AmountMinor: 101, OccurredAt: time.Now().UTC(), ProviderReference: "verified-original-chargeback"}
			if err := m.RecordChargebackSettlement(ctx, cb); err != nil {
				t.Fatal(err)
			}
			if err := m.RecordChargebackSettlement(ctx, cb); err != nil {
				t.Fatal(err)
			}
			c := e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: kind, ID: req.ID, Version: req.Version, Delivery: &e.Delivery{Content: "complete"}, DeliveryVersion: 1, Reason: "missing document"}
			if kind == "accept" || kind == "reject" {
				c.Scope.OrganizationID = "buyer"
			}
			if _, err := s.Mutate(ctx, c); !errors.Is(err, e.ErrConflict) {
				t.Fatalf("canonical chargeback allowed %s: %v", kind, err)
			}
			page, err := r.Read(ctx, e.Query{Scope: c.Scope, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
			if err != nil || !page.Requests[0].FinancialFence || page.Requests[0].FinancialState != "RECONCILIATION_REQUIRED" || page.Requests[0].State != req.State || page.Requests[0].AcceptanceID != "" {
				t.Fatalf("chargeback not durably projected: %+v %v", page, err)
			}
			var commands int64
			r.db.Model(&financialRow{}).Count(&commands)
			if commands != 1 {
				t.Fatal("fulfillment check created a new financial command", commands)
			}
			command, err := r.OriginalFinancialCommand(ctx, req.OrderID)
			if err != nil || command.ID != original.ID {
				t.Fatal("original identity changed", err)
			}
		})
	}
}

func TestPaidOriginalRecoveryProjectsChargebackBeforeExpiry(t *testing.T) {
	r, s, m, _, b, req, _ := fulfillmentFundsFixture(t)
	ctx := context.Background()
	order, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RecordChargebackSettlement(ctx, money.ChargebackSettlement{ChargebackID: "signed-chargeback", PaymentID: order.MoneyInput().Payment.PaymentID, AmountMinor: 101, OccurredAt: time.Now().UTC(), ProviderReference: "verified-original-chargeback"}); err != nil {
		t.Fatal(err)
	}
	// The existing durable original-command loop must discover a paid order
	// before its freeze expiry, without a new money -> ecosystem event system.
	r.db.Model(&financialRow{}).Where("order_id=?", req.OrderID).Update("next_attempt_at", time.Time{})
	if err := s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	page, err := r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer"}, Kind: "requests", ID: req.ID, Page: 1, PageSize: 1})
	if err != nil || !page.Requests[0].FinancialFence || page.Requests[0].FinancialReason != "CHANNEL_CHARGEBACK_REQUIRES_RECONCILIATION" {
		t.Fatalf("pre-expiry recovery hid chargeback: %+v %v", page, err)
	}
}
