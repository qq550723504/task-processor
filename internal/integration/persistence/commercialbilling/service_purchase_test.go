package commercialbilling

import (
	"context"
	"errors"
	"task-processor/internal/commercial/billing"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

type serviceSourceFixture struct {
	original       billing.ServicePurchaseCommand
	cancel, denied bool
	denySettle     bool
}

func TestServiceRefundableAmountUsesCanonicalCumulativeRefund(t *testing.T) {
	svc, _, _, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Execute(ctx, serviceCommand(src, "REFUND", "first-partial-refund", 60)); err != nil {
		t.Fatal(err)
	}
	remaining, err := svc.ReadServiceRefundableAmount(ctx, src.original.OrderID)
	if err != nil || remaining != 41 {
		t.Fatalf("remaining refundable amount=%d err=%v", remaining, err)
	}
}

func (f *serviceSourceFixture) OriginalServicePurchase(context.Context, string) (billing.ServicePurchaseCommand, error) {
	return f.original, nil
}
func (f *serviceSourceFixture) VerifyServiceCommand(context.Context, billing.ServicePurchaseCommand) error {
	return nil
}
func (f *serviceSourceFixture) CanDispatchServiceCommand(context.Context, billing.ServicePurchaseCommand) error {
	return nil
}
func (f *serviceSourceFixture) AdmitServiceCommand(_ context.Context, c billing.ServicePurchaseCommand, operation string) error {
	if f.denySettle && c.Kind == "SETTLE" {
		return billing.ErrOrderCancelled
	}
	if c.Kind == "CREATE_PURCHASE" && f.cancel {
		return billing.ErrOrderCancelled
	}
	return nil
}
func TestServiceUndispatchedSettlementDenialDoesNotBlockApprovedRefund(t *testing.T) {
	svc, _, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	src.denySettle = true
	_, _ = svc.Execute(ctx, serviceCommand(src, "SETTLE", "accepted-before-dispute", 101))
	result, err := svc.Execute(ctx, serviceCommand(src, "REFUND", "approved-dispute", 2))
	if err != nil || result.State != "REFUNDED" {
		t.Fatalf("denied undispatched settlement blocked refund: %+v %v", result, err)
	}
	src.denySettle = false
	result, err = svc.Execute(ctx, serviceCommand(src, "SETTLE", "accepted-before-dispute", 101))
	if err != nil || result.State != "SETTLED" {
		t.Fatalf("original acceptance didn't resume at current net: %+v %v", result, err)
	}
	f, _ := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if f.SharedMinor != 9 || f.ReleasedMinor != 90 {
		t.Fatalf("resumed settlement wrong net %+v", f)
	}
}
func (f *serviceSourceFixture) AuthorizeServiceCheckout(_ context.Context, org, actor, order string) error {
	if f.cancel {
		return billing.ErrOrderCancelled
	}
	if f.denied || org != f.original.BuyerOrganizationID || actor != f.original.ActorID {
		return billing.ErrAuthorizationRevoked
	}
	return nil
}

type serviceProtectionFixture struct{}

func (serviceProtectionFixture) Seal(k, v string) ([]byte, error) { return []byte(k + "|" + v), nil }
func (serviceProtectionFixture) Open(k string, v []byte) (string, error) {
	if len(v) <= len(k) || string(v[:len(k)+1]) != k+"|" {
		return "", billing.ErrConflict
	}
	return string(v[len(k)+1:]), nil
}

type serviceProviderFixture struct {
	unsplitQueries                                         int
	unsplitMinor                                           *int64
	unsplitError                                           bool
	paidAt                                                 time.Time
	replayAbsent                                           bool
	profile                                                billing.ServiceMerchantProfile
	enabled, paid, lostCheckout, lostEffect, refundPending bool
	refundUnknown, failShare                               bool
	creates, closes                                        int
	dispatched                                             map[string]int
	effects                                                map[string]billing.ServiceOperationObservation
}

func (f *serviceProviderFixture) QueryServiceUnsplit(_ context.Context, o billing.ServicePurchaseOrder) (billing.ServiceUnsplitObservation, error) {
	f.unsplitQueries++
	if f.unsplitError {
		return billing.ServiceUnsplitObservation{}, billing.ErrReconciliationRequired
	}
	amount := o.Source.AmountMinor
	for _, effect := range f.effects {
		if effect.State == "SUCCESS" && (effect.Kind == money.ServiceShare || effect.Kind == money.ServiceFinish || effect.Kind == money.ServiceRefundRelease || effect.Kind == money.ServiceRefund) {
			amount -= effect.AmountMinor
		}
	}
	if amount < 0 {
		amount = 0
	}
	if f.unsplitMinor != nil {
		amount = *f.unsplitMinor
	}
	return billing.ServiceUnsplitObservation{ProfileVersion: o.Profile.Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: o.Payment.TransactionID, UnsplitMinor: amount, ProofID: "verified-unsplit", VerificationVersion: "fixture-verified", OccurredAt: time.Now().UTC()}, nil
}

func TestServiceExpiredPaidOriginalBypassesCachedResultAndFencesChangedFunds(t *testing.T) {
	svc, _, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	p.paidAt = time.Now().AddDate(0, 0, -181)
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	p.unsplitMinor = &zero
	result, err := svc.Execute(ctx, src.original)
	if err != nil || p.unsplitQueries != 1 || result.State != "RECONCILIATION_REQUIRED" {
		t.Fatalf("cached payment hid channel change: %+v %v queries=%d", result, err, p.unsplitQueries)
	}
	got, _ := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if got.AutomaticReleasedMinor != 0 || got.ReconciliationReason == "" {
		t.Fatalf("invented channel release %+v", got)
	}
	_, _ = svc.Execute(ctx, serviceCommand(src, "SETTLE", "expired-accept", 101))
	if len(p.dispatched) != 0 {
		t.Fatal("expired changed funds dispatched a new share")
	}
}

func TestServiceExpiredOriginalWaitsForLostShareReadback(t *testing.T) {
	svc, _, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	p.paidAt = time.Now().UTC().AddDate(0, 0, -181)
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	p.lostEffect = true
	c := serviceCommand(src, "SETTLE", "expired-original-acceptance", 101)
	if _, err := svc.Execute(ctx, c); err == nil {
		t.Fatal("lost share acknowledgement not exercised")
	}
	p.unsplitQueries = 0
	if _, err := svc.Execute(ctx, src.original); err != nil || p.unsplitQueries != 0 {
		t.Fatal("cached original inferred release from the pending share", err)
	}
	result, err := svc.Execute(ctx, c)
	if err != nil || result.State != "SETTLED" {
		t.Fatalf("original readback blocked by expiry: %+v %v", result, err)
	}
	f, _ := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if f.ReconciliationReason != "" || f.SharedMinor != 10 || f.ReleasedMinor != 91 {
		t.Fatalf("original recovery invented an anomaly: %+v", f)
	}
}

func TestServiceExpiredUnknownBalanceCannotDispatchNewOperation(t *testing.T) {
	svc, _, _, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	p.paidAt = time.Now().UTC().AddDate(0, 0, -181)
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	p.unsplitError = true
	if _, err := svc.Execute(ctx, serviceCommand(src, "SETTLE", "unknown-expired-acceptance", 101)); err == nil {
		t.Fatal("unknown balance admitted")
	}
	if len(p.dispatched) != 0 {
		t.Fatal("unknown original balance dispatched share")
	}
}

func (f *serviceProviderFixture) Profile() billing.ServiceMerchantProfile { return f.profile }
func (f *serviceProviderFixture) NewPaymentsEnabled() bool                { return f.enabled }
func (f *serviceProviderFixture) CreateServiceCheckout(context.Context, billing.ServicePurchaseOrder) (string, error) {
	f.creates++
	if f.lostCheckout {
		return "", errors.New("lost acknowledgement")
	}
	return "weixin://wxpay/bizpayurl?pr=fixture", nil
}
func (f *serviceProviderFixture) QueryServicePayment(_ context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	p := billing.ServicePaymentObservation{EventID: "unpaid:" + o.TradeNo, ProfileVersion: o.Profile.Version, PlatformMerchantID: o.Profile.PlatformMerchantID, AppID: o.Profile.AppID, ProviderMerchantID: o.Source.ProviderMerchantID, TradeNo: o.TradeNo, Currency: "CNY", State: "UNPAID", VerificationVersion: "fixture-verified"}
	if f.paid {
		p.EventID = "paid:" + o.TradeNo
		p.State = "PAID"
		if f.refundUnknown {
			p.State = "PAID_REFUND_UNKNOWN"
			p.EventID = "refund-unknown:" + o.TradeNo
		}
		p.TransactionID = "original-channel-payment"
		p.AmountMinor = o.Source.AmountMinor
		p.OccurredAt = time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC)
		if !f.paidAt.IsZero() {
			p.OccurredAt = f.paidAt
		}
	}
	return p, nil
}
func (f *serviceProviderFixture) CloseServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	f.closes++
	p, _ := f.QueryServicePayment(ctx, o)
	if p.State != "PAID" {
		p.State = "CLOSED"
		p.EventID = "closed:" + o.TradeNo
	}
	return p, nil
}
func (f *serviceProviderFixture) DispatchServiceOperation(_ context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	f.dispatched[op.ProviderRequestID]++
	p := billing.ServiceOperationObservation{EventID: "effect:" + op.ProviderRequestID, ProfileVersion: o.Profile.Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: o.Payment.TransactionID, ProviderRequestID: op.ProviderRequestID, Kind: op.Reservation.Kind, AmountMinor: op.Reservation.AmountMinor, State: "SUCCESS", ProviderReference: "original-channel-effect:" + op.ProviderRequestID, VerificationVersion: "fixture-verified", OccurredAt: time.Date(2026, 10, 8, 1, 1, 0, 0, time.UTC)}
	if f.refundPending && p.Kind == money.ServiceRefund {
		p.State = "WAITING_FUNDS"
		p.Reason = "ORIGINAL_SUBMERCHANT_FUNDS_INSUFFICIENT"
	}
	f.effects[op.ProviderRequestID] = p
	if f.failShare && p.Kind == money.ServiceShare {
		p.State = "FAILED"
		p.Reason = "ORIGINAL_SHARE_CLOSED"
		f.effects[op.ProviderRequestID] = p
	}
	if f.lostEffect {
		f.lostEffect = false
		return billing.ServiceOperationObservation{}, errors.New("lost acknowledgement")
	}
	return p, nil
}

type loseFailedBillingSave struct {
	*Repository
	lost bool
}

func (r *loseFailedBillingSave) SaveServicePurchase(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePurchaseOrder, error) {
	if o.State == "CHANNEL_OPERATION_FAILED" && !r.lost {
		r.lost = true
		return billing.ServicePurchaseOrder{}, errors.New("B7 write acknowledgement lost")
	}
	return r.Repository.SaveServicePurchase(ctx, o)
}
func TestServiceFailedMoneyReceiptRecoversLostBillingSaveAndAllowsRefund(t *testing.T) {
	_, r, funds, src, p := servicePurchaseFixture(t)
	store := &loseFailedBillingSave{Repository: r}
	svc, err := billing.NewServicePurchases(store, funds, p, src, serviceProtectionFixture{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p.paid = true
	p.failShare = true
	if _, err = svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	accepted := serviceCommand(src, "SETTLE", "accept-with-channel-failure", 101)
	if _, err = svc.Execute(ctx, accepted); err == nil || !store.lost {
		t.Fatalf("did not exercise M3 before lost B7: %v", err)
	}
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.PendingOperationID != "" || f.SharedMinor != 0 {
		t.Fatalf("failure was not durably finalized: %+v %v", f, err)
	}
	result, err := svc.Execute(ctx, accepted)
	if err != nil || result.State != "CHANNEL_OPERATION_FAILED" || result.ReceiptID == "" {
		t.Fatalf("lost B7 cannot recover original failed receipt: %+v %v", result, err)
	}
	result, err = svc.Execute(ctx, serviceCommand(src, "REFUND", "approved-after-failed-share", 101))
	if err != nil || result.State != "REFUNDED" {
		t.Fatalf("failed original command blocks approved refund: %+v %v", result, err)
	}
	for id, count := range p.dispatched {
		if count != 1 {
			t.Fatalf("repeated channel operation %s: %d", id, count)
		}
	}
}
func TestServicePaymentRefundUnknownFencesOriginalMoneyAndNewFinancialEffects(t *testing.T) {
	svc, r, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	p.refundUnknown = true
	result, err := svc.Execute(ctx, src.original)
	if err != nil || result.PaymentReceiptID == "" || result.State != "RECONCILIATION_REQUIRED" {
		t.Fatalf("original paid fact lost: %+v %v", result, err)
	}
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.ReconciliationReason == "" {
		t.Fatalf("refund uncertainty lacks canonical money fence: %+v %v", f, err)
	}
	for _, kind := range []string{"SETTLE", "REFUND"} {
		result, _ = svc.Execute(ctx, serviceCommand(src, kind, "blocked-"+kind, 101))
		if result.State != "RECONCILIATION_REQUIRED" {
			t.Fatalf("unknown refund allowed %s: %+v", kind, result)
		}
	}
	o, _ := r.ReadServicePurchase(ctx, src.original.OrderID)
	if len(p.dispatched) != 0 || len(o.Operations) != 0 {
		t.Fatalf("unknown net amount created new financial effects: %+v", o.Operations)
	}
}
func TestServiceRefundUncertaintyStillRecoversDispatchedOriginalShare(t *testing.T) {
	svc, r, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	p.lostEffect = true
	accepted := serviceCommand(src, "SETTLE", "original-acceptance-before-uncertainty", 101)
	if _, err := svc.Execute(ctx, accepted); err == nil {
		t.Fatal("lost response did not interrupt")
	}
	o, _ := r.ReadServicePurchase(ctx, src.original.OrderID)
	p.refundUnknown = true
	observation, _ := p.QueryServicePayment(ctx, o)
	if err := r.RecordServicePaymentObservation(ctx, o, observation); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Execute(ctx, accepted)
	if err != nil || result.State != "RECONCILIATION_REQUIRED" {
		t.Fatalf("did not retain uncertainty: %+v %v", result, err)
	}
	f, _ := funds.ReadServiceFunds(ctx, src.original.OrderID)
	o, _ = r.ReadServicePurchase(ctx, src.original.OrderID)
	if f.SharedMinor != 10 || f.PendingOperationID != "" || f.ReconciliationReason == "" || len(o.Effects) != 1 {
		t.Fatalf("original dispatched fact cannot recover under fence: %+v effects=%+v", f, o.Effects)
	}
	if len(p.dispatched) != 1 {
		t.Fatalf("new finish dispatched under uncertainty: %+v", p.dispatched)
	}
	for id, count := range p.dispatched {
		if count != 1 {
			t.Fatalf("original share replayed %s=%d", id, count)
		}
	}
}
func (f *serviceProviderFixture) QueryServiceOperation(_ context.Context, _ billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	p, ok := f.effects[op.ProviderRequestID]
	if !ok {
		return p, billing.ErrReconciliationRequired
	}
	if f.replayAbsent {
		p.State = "REPLAY_ALLOWED"
		p.EventID = "verified-original-absent:" + op.ProviderRequestID
	}
	return p, nil
}
func TestServiceOriginalAbsentCannotReplayThroughNewMoneyFence(t *testing.T) {
	svc, r, _, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	p.lostEffect = true
	accepted := serviceCommand(src, "SETTLE", "original-acceptance-absent", 101)
	if _, err := svc.Execute(ctx, accepted); err == nil {
		t.Fatal("lost response did not interrupt")
	}
	o, _ := r.ReadServicePurchase(ctx, src.original.OrderID)
	p.refundUnknown = true
	p.replayAbsent = true
	observation, _ := p.QueryServicePayment(ctx, o)
	if err := r.RecordServicePaymentObservation(ctx, o, observation); err != nil {
		t.Fatal(err)
	}
	_, _ = svc.Execute(ctx, accepted)
	for id, count := range p.dispatched {
		if count != 1 {
			t.Fatalf("original absent replay bypassed new money fence %s=%d", id, count)
		}
	}
}
func servicePurchaseFixture(t *testing.T) (*billing.ServicePurchases, *Repository, *moneystore.Repository, *serviceSourceFixture, *serviceProviderFixture) {
	t.Helper()
	r := commercialRepository(t)
	r.now = func() time.Time { return time.Now().UTC() }
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	funds, err := moneystore.New(r.db)
	if err != nil {
		t.Fatal(err)
	}
	src := &serviceSourceFixture{original: billing.ServicePurchaseCommand{Allocation: money.ServiceAllocationPolicy{CommissionBPS: 1000, Basis: money.ServiceAllocationCumulativeNetFloorV1}, ID: "create-proof", RequestID: "original-request", OrderID: "original-service-order", Kind: "CREATE_PURCHASE", SourceProofID: "create-proof", ActorID: "buyer-member", BuyerOrganizationID: "buyer-org", ProviderOrganizationID: "provider-org", ProviderMerchantID: "sub-merchant", QuoteVersion: 1, AmountMinor: 101, DeliveryDays: 7, PolicyVersion: "eco-10-platform-fee-ar1"}}
	p := &serviceProviderFixture{profile: billing.ServiceMerchantProfile{Version: "original-profile", Environment: "PRODUCTION", PlatformMerchantID: "platform-merchant", AppID: "platform-app", FreezeDays: 180}, enabled: true, dispatched: map[string]int{}, effects: map[string]billing.ServiceOperationObservation{}}
	svc, err := billing.NewServicePurchases(r, funds, p, src, serviceProtectionFixture{})
	if err != nil {
		t.Fatal(err)
	}
	return svc, r, funds, src, p
}
func serviceCommand(src *serviceSourceFixture, kind, id string, amount int64) billing.ServicePurchaseCommand {
	c := src.original
	c.ID = id
	c.SourceProofID = id
	c.Kind = kind
	c.AmountMinor = amount
	return c
}
func TestServiceCancelBeforeCheckoutAndLostCheckoutNeverResends(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "before", false: "lost"}[before], func(t *testing.T) {
			svc, _, funds, src, p := servicePurchaseFixture(t)
			ctx := context.Background()
			if _, err := svc.Execute(ctx, src.original); err != nil {
				t.Fatal(err)
			}
			if !before {
				p.lostCheckout = true
				if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err == nil {
					t.Fatal("lost checkout reported success")
				}
				if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err == nil {
					t.Fatal("unknown checkout regenerated")
				}
				if p.creates != 1 {
					t.Fatalf("duplicate checkout %d", p.creates)
				}
			}
			src.cancel = true
			if !before {
				p.paid = true
			}
			result, err := svc.Execute(ctx, serviceCommand(src, "CANCEL", "original-cancel", 101))
			if err != nil {
				t.Fatal(err)
			}
			want := "CLOSED_UNPAID"
			if !before {
				want = "REFUNDED"
			}
			if result.State != want {
				t.Fatalf("cancel %s: %+v", want, result)
			}
			if _, err = svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err == nil {
				t.Fatal("cancelled order exposed QR")
			}
			if before && p.creates != 0 {
				t.Fatal("cancel-before-checkout dispatched payment")
			}
			if !before {
				f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
				if err != nil || f.RefundedMinor != 101 {
					t.Fatalf("late payment not fully refunded: %+v %v", f, err)
				}
			}
		})
	}
}
func TestServiceLostShareReadbackThenReturnBeforeRefund(t *testing.T) {
	svc, r, funds, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	p.paid = true
	result, err := svc.Execute(ctx, src.original)
	if err != nil || result.PaymentReceiptID == "" {
		t.Fatalf("payment not accepted: %+v %v", result, err)
	}
	p.lostEffect = true
	accepted := serviceCommand(src, "SETTLE", "customer-accepted-version-1", 101)
	_, _ = svc.Execute(ctx, accepted)
	result, err = svc.Execute(ctx, accepted)
	if err != nil || result.State != "SETTLED" {
		t.Fatalf("original share readback: %+v %v", result, err)
	}
	for id, count := range p.dispatched {
		if count != 1 {
			t.Fatalf("repeated dispatch %s=%d", id, count)
		}
	}
	o, err := r.ReadServicePurchase(ctx, src.original.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Effects) != 2 {
		t.Fatalf("share and finish receipts missing %+v", o.Effects)
	}
	p.refundPending = true
	approved := serviceCommand(src, "REFUND", "refund-both-confirmed-platform-approved", 2)
	result, err = svc.Execute(ctx, approved)
	if err != nil || result.State != "WAITING_FUNDS" {
		t.Fatalf("refund must retain original insufficient funds: %+v %v", result, err)
	}
	f, err := funds.ReadServiceFunds(ctx, src.original.OrderID)
	if err != nil || f.ReturnedMinor != 1 || f.RefundedMinor != 0 || f.PlatformMinor != 10 {
		t.Fatalf("return isn't customer refund: %+v %v", f, err)
	}
	p.refundPending = false
	for id, e := range p.effects {
		if e.Kind == money.ServiceRefund {
			e.State = "SUCCESS"
			e.EventID = "success:" + id
			p.effects[id] = e
		}
	}
	result, err = svc.Execute(ctx, approved)
	if err != nil || result.State != "REFUNDED" {
		t.Fatalf("refund original identity: %+v %v", result, err)
	}
	f, _ = funds.ReadServiceFunds(ctx, src.original.OrderID)
	if f.PlatformMinor != 9 || f.ProviderMinor != 90 || f.RefundedMinor != 2 {
		t.Fatalf("incorrect cumulative allocation %+v", f)
	}
	p.enabled = false
	result, err = svc.Execute(ctx, approved)
	if err != nil || result.State != "REFUNDED" {
		t.Fatalf("payments-off disabled original readback %+v %v", result, err)
	}
	for id, count := range p.dispatched {
		if count != 1 {
			t.Fatalf("repeated operation %s=%d", id, count)
		}
	}
}
func TestServiceCheckoutFreshAuthorizationAndOriginalProfile(t *testing.T) {
	svc, _, _, src, p := servicePurchaseFixture(t)
	ctx := context.Background()
	if _, err := svc.Execute(ctx, src.original); err != nil {
		t.Fatal(err)
	}
	src.denied = true
	if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); !errors.Is(err, billing.ErrAuthorizationRevoked) {
		t.Fatalf("revoked member %v", err)
	}
	if p.creates != 0 {
		t.Fatal("revoked member generated checkout")
	}
	src.denied = false
	p.profile.Version = "replacement-profile"
	if _, err := svc.Checkout(ctx, "buyer-org", "buyer-member", src.original.OrderID); err == nil {
		t.Fatal("original order switched merchant profile")
	}
	if p.creates != 0 {
		t.Fatal("replacement profile generated checkout")
	}
}
