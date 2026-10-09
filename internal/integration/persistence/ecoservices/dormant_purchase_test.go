package ecoservices

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
)

type dormantCheckoutProvider struct {
	fulfillmentPaymentProvider
	creates int
}

func (*dormantCheckoutProvider) NewPaymentsEnabled() bool { return true }
func (p *dormantCheckoutProvider) CreateServiceCheckout(context.Context, billing.ServicePurchaseOrder) (string, error) {
	p.creates++
	return "weixin://wxpay/bizpayurl?pr=original-fixture", nil
}

type originalBuyerCheckoutAuthorization struct{}

func (originalBuyerCheckoutAuthorization) AuthorizeServicePurchase(_ context.Context, org, actor string) error {
	if org != "buyer" || actor != "buyer" {
		return billing.ErrAuthorizationRevoked
	}
	return nil
}

type dormantCheckoutProtection struct{}

func (dormantCheckoutProtection) Seal(_ string, value string) ([]byte, error) {
	return []byte(value), nil
}
func (dormantCheckoutProtection) Open(_ string, value []byte) (string, error) {
	return string(value), nil
}

func TestFirstCheckoutMaterializesDormantOriginalWithoutPriorRecovery(t *testing.T) {
	r, _, m, _, b, _, _ := fulfillmentFundsFixture(t)
	req, original := checkoutAdmissionFixture(t, r)
	p := &dormantCheckoutProvider{}
	purchases, err := billing.NewServicePurchases(b, m, p, ecoservicesbilling.Source{Repository: r, Authorizer: originalBuyerCheckoutAuthorization{}}, dormantCheckoutProtection{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := b.ReadServicePurchase(ctx, req.OrderID); err != billing.ErrNotFound {
		t.Fatal("dormant purchase already materialized", err)
	}
	qr, err := purchases.Checkout(ctx, "buyer", "buyer", req.OrderID)
	if err != nil || qr != "weixin://wxpay/bizpayurl?pr=original-fixture" || p.creates != 1 {
		t.Fatalf("first checkout requires prior recovery: %q %v creates=%d", qr, err, p.creates)
	}
	o, err := b.ReadServicePurchase(ctx, req.OrderID)
	if err != nil || !o.PaymentDispatched || o.Source.ID != original.ID {
		t.Fatal("original dispatch identity missing", err)
	}
	commands, err := r.PendingFinancialCommands(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range commands {
		found = found || c.ID == original.ID
	}
	if !found {
		t.Fatal("checkout did not wake original recovery")
	}
}

type dormantPurchaseTrading struct {
	e.TradingPort
	calls int
}

func (t *dormantPurchaseTrading) ExecuteServiceCommand(context.Context, e.FinancialCommand) (e.FinancialResult, error) {
	t.calls++
	return e.FinancialResult{}, e.ErrUnavailable
}

func TestUntouchedPurchaseDormantUntilOriginalAdmissionOrVerifiedWake(t *testing.T) {
	for _, trigger := range []string{"checkout-admission", "verified-notification"} {
		t.Run(trigger, func(t *testing.T) {
			r, _ := fixture(t)
			req, original := checkoutAdmissionFixture(t, r)
			ctx := context.Background()
			trading := &dormantPurchaseTrading{}
			s, err := e.NewService(r, trading, 180)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := s.Recover(ctx); err != nil {
					t.Fatal(err)
				}
				commands, err := r.PendingFinancialCommands(ctx, 20)
				if err != nil || len(commands) != 0 {
					t.Fatalf("untouched quote entered channel recovery: %+v %v", commands, err)
				}
			}
			if trading.calls != 0 {
				t.Fatal("untouched quote called the financial provider", trading.calls)
			}
			if trigger == "checkout-admission" {
				in := original
				in.DispatchOperationID = "checkout:" + in.ID
				if _, err := r.AdmitFinancialCommand(ctx, in); err != nil {
					t.Fatal(err)
				}
			} else if err := r.WakeOriginalServicePurchase(ctx, req.OrderID); err != nil {
				t.Fatal(err)
			}
			commands, err := r.PendingFinancialCommands(ctx, 20)
			if err != nil || len(commands) != 1 || commands[0].ID != original.ID {
				t.Fatalf("original recovery was not woken: %+v %v", commands, err)
			}
			if err := r.CompleteFinancialCommand(ctx, commands[0], e.FinancialResult{OrderID: req.OrderID, State: "AWAITING_PAYMENT", Revision: 1}); err != nil {
				t.Fatal(err)
			}
			var count int64
			r.db.Model(&financialRow{}).Where("order_id=?", req.OrderID).Count(&count)
			if count != 1 {
				t.Fatal("wake created a replacement purchase", count)
			}
		})
	}
}

func TestUntouchedPurchasesDoNotHidePendingFinancialCommands(t *testing.T) {
	r, _ := fixture(t)
	for i := 0; i < 21; i++ {
		checkoutAdmissionFixture(t, r)
	}
	for _, kind := range []string{"SETTLE", "REFUND", "CANCEL"} {
		_, c := checkoutAdmissionFixture(t, r)
		c.ID, c.Kind = uuid.NewString(), kind
		c.SourceProofID = c.ID
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.db.Create(&financialRow{ID: c.ID, RequestID: c.RequestID, OrderID: c.OrderID, Kind: kind, Fingerprint: e.Fingerprint(c), Payload: raw, State: "PENDING", CreatedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
		commands, err := r.PendingFinancialCommands(context.Background(), 1)
		if err != nil || len(commands) != 1 || commands[0].ID != c.ID {
			t.Fatalf("untouched backlog hid %s: %+v %v", kind, commands, err)
		}
	}
}
