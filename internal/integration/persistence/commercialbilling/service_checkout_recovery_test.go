package commercialbilling

import (
	"context"
	"errors"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
	"testing"
)

type checkoutProtectionFault struct {
	serviceProtectionFixture
	afterOpen func()
	failSeal  bool
}

func (f *checkoutProtectionFault) Open(k string, v []byte) (string, error) {
	qr, err := f.serviceProtectionFixture.Open(k, v)
	if f.afterOpen != nil {
		f.afterOpen()
	}
	return qr, err
}
func (f *checkoutProtectionFault) Seal(k, v string) ([]byte, error) {
	if f.failSeal {
		f.failSeal = false
		return nil, errors.New("local seal unavailable")
	}
	return f.serviceProtectionFixture.Seal(k, v)
}

type checkoutSaveFault struct {
	billing.ServicePurchaseStore
	failOnce bool
}

func (f *checkoutSaveFault) SaveServicePurchase(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePurchaseOrder, error) {
	if f.failOnce && len(o.CheckoutCiphertext) > 0 {
		f.failOnce = false
		return o, errors.New("checkout save unavailable")
	}
	return f.ServicePurchaseStore.SaveServicePurchase(ctx, o)
}

type checkoutProviderProof struct {
	*serviceProviderFixture
	orders      []billing.ServicePurchaseOrder
	queries     int
	queryError  bool
	proof       string
	afterQuery  func()
	badAmount   bool
	badIdentity bool
	badCurrency bool
	closed      bool
}

func (f *checkoutProviderProof) CreateServiceCheckout(ctx context.Context, o billing.ServicePurchaseOrder) (string, error) {
	f.orders = append(f.orders, o)
	return f.serviceProviderFixture.CreateServiceCheckout(ctx, o)
}
func (f *checkoutProviderProof) QueryServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	f.queries++
	if f.queryError {
		return billing.ServicePaymentObservation{}, billing.ErrReconciliationRequired
	}
	p, err := f.serviceProviderFixture.QueryServicePayment(ctx, o)
	if p.State == "UNPAID" {
		p.AmountMinor = o.Source.AmountMinor
		p.CheckoutRecovery = f.proof
		if f.proof == billing.ServiceCheckoutAbsentProof {
			p.AmountMinor = 0
		}
	}
	if f.badAmount {
		p.AmountMinor++
	}
	if f.badIdentity {
		p.TradeNo = "other-original-trade"
	}
	if f.badCurrency {
		p.Currency = "USD"
	}
	if f.closed {
		p.State = "CLOSED"
	}
	// Like the real signed adapter, changed query facts have distinct inbox
	// identities so a conflict with an earlier fixture cannot mask admission.
	p.EventID = "checkout-query:" + money.ServiceFingerprint(p)
	if f.afterQuery != nil {
		f.afterQuery()
	}
	return p, err
}

func TestServiceCachedCheckoutRechecksCancellationAfterDecryption(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "revoked"}[revoked], func(t *testing.T) {
			svc, store, funds, src, p := servicePurchaseFixture(t)
			ctx := context.Background()
			if _, err := svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err != nil {
				t.Fatal(err)
			}
			protection := &checkoutProtectionFault{afterOpen: func() {
				if revoked {
					src.denied = true
				} else {
					src.cancel = true
				}
			}}
			svc, err := billing.NewServicePurchases(store, funds, p, src, protection)
			if err != nil {
				t.Fatal(err)
			}
			qr, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID)
			if err == nil || qr != "" {
				t.Fatalf("cached QR disclosed after cancellation/revocation: %q %v", qr, err)
			}
			if p.creates != 1 {
				t.Fatalf("cached read created another order: %d", p.creates)
			}
		})
	}
}

func TestServiceCheckoutRecoversOriginalAfterResponseSealOrSaveLoss(t *testing.T) {
	for _, failure := range []string{"response", "absence", "seal", "save"} {
		t.Run(failure, func(t *testing.T) {
			_, store, funds, src, p := servicePurchaseFixture(t)
			ctx := context.Background()
			provider := &checkoutProviderProof{serviceProviderFixture: p, proof: "NATIVE_UNPAID_MATCHED"}
			if failure == "absence" {
				provider.proof = billing.ServiceCheckoutAbsentProof
			}
			protection := &checkoutProtectionFault{failSeal: failure == "seal"}
			faultStore := &checkoutSaveFault{ServicePurchaseStore: store, failOnce: failure == "save"}
			svc, err := billing.NewServicePurchases(faultStore, funds, provider, src, protection)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			p.lostCheckout = failure == "response" || failure == "absence"
			if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err == nil {
				t.Fatal("lost checkout returned success")
			}
			p.lostCheckout = false
			qr, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID)
			if err != nil || qr == "" {
				t.Fatalf("verified original unpaid checkout did not recover: %q %v", qr, err)
			}
			if len(provider.orders) != 2 || provider.orders[0].Fingerprint() != provider.orders[1].Fingerprint() {
				t.Fatalf("recovery changed original order or parameters: %+v", provider.orders)
			}
			o, err := store.ReadServicePurchase(ctx, src.original.OrderID)
			if err != nil || !o.PaymentDispatched || len(o.CheckoutCiphertext) == 0 {
				t.Fatalf("original fence/QR not retained: %+v %v", o, err)
			}
			observations, err := store.ServicePaymentObservations(ctx, src.original.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, observation := range observations {
				if observation.CheckoutRecovery == provider.proof && observation.AllowsCheckoutReplay(o) {
					found = true
				}
			}
			if !found {
				t.Fatal("fresh original recovery proof was not retained")
			}
			creates := p.creates
			if _, err = svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err != nil || p.creates != creates {
				t.Fatalf("cached recovered code regenerated: %v creates=%d", err, p.creates)
			}
		})
	}
}

func TestServiceCheckoutRecoveryRequiresFreshMatchedProofAndLiveAdmission(t *testing.T) {
	for _, failure := range []string{"unknown", "no-proof", "wrong-amount", "wrong-identity", "wrong-currency", "closed", "paid", "cancel", "revoked", "disabled"} {
		t.Run(failure, func(t *testing.T) {
			_, store, funds, src, p := servicePurchaseFixture(t)
			ctx := context.Background()
			provider := &checkoutProviderProof{serviceProviderFixture: p, proof: "NATIVE_UNPAID_MATCHED"}
			svc, err := billing.NewServicePurchases(store, funds, provider, src, serviceProtectionFixture{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			p.lostCheckout = true
			_, _ = svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID)
			p.lostCheckout = false
			switch failure {
			case "unknown":
				provider.queryError = true
			case "no-proof":
				provider.proof = ""
			case "wrong-amount":
				provider.badAmount = true
			case "wrong-identity":
				provider.badIdentity = true
			case "wrong-currency":
				provider.badCurrency = true
			case "closed":
				provider.closed = true
			case "paid":
				p.paid = true
			case "cancel":
				provider.afterQuery = func() { src.cancel = true }
			case "revoked":
				provider.afterQuery = func() { src.denied = true }
			case "disabled":
				provider.afterQuery = func() { p.enabled = false }
			}
			qr, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID)
			if err == nil || qr != "" || p.creates != 1 {
				t.Fatalf("unsafe checkout recovery: qr=%q err=%v creates=%d", qr, err, p.creates)
			}
		})
	}
}
