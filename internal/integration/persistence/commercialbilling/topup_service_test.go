package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
	moneystore "task-processor/internal/integration/persistence/money"
)

type topUpTestAuthorizer struct {
	denied        bool
	calls, denyAt int
}

func (a *topUpTestAuthorizer) AuthorizeTopUp(context.Context, string, string, bool) error {
	a.calls++
	if a.denied || (a.denyAt > 0 && a.calls >= a.denyAt) {
		return billing.ErrAuthorizationRevoked
	}
	return nil
}

type topUpTestProtection struct{}

func (topUpTestProtection) Seal(binding, value string) ([]byte, error) {
	return []byte(binding + "|" + value), nil
}
func (topUpTestProtection) Open(binding string, value []byte) (string, error) {
	if len(value) < len(binding)+1 || string(value[:len(binding)+1]) != binding+"|" {
		return "", billing.ErrConflict
	}
	return string(value[len(binding)+1:]), nil
}

type topUpTestProvider struct {
	merchant          billing.TopUpMerchant
	checkouts         int
	lost              bool
	observation       billing.ProviderObservation
	refundObservation billing.ProviderObservation
}

func (p *topUpTestProvider) Merchant() billing.TopUpMerchant { return p.merchant }
func (p *topUpTestProvider) Available() bool                 { return true }
func (p *topUpTestProvider) CreateOrReadCheckout(_ context.Context, a billing.TopUpPaymentAttempt) (billing.CheckoutAction, error) {
	p.checkouts++
	if p.lost {
		return billing.CheckoutAction{}, errors.New("response lost")
	}
	kind := "REDIRECT"
	if p.merchant.Provider == billing.PaymentWeChat {
		kind = "QR_CODE"
	}
	return billing.CheckoutAction{Kind: kind, Payload: "fixture-payload", OrderID: a.OrderID, AttemptID: a.AttemptID, Provider: p.merchant.Provider, ExpiresAt: a.ExpiresAt, RequestFingerprint: a.Fingerprint()}, nil
}
func (p *topUpTestProvider) QueryPayment(context.Context, billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	return p.observation, nil
}
func (p *topUpTestProvider) ClosePayment(context.Context, billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	return billing.ProviderObservation{}, billing.ErrReconciliationRequired
}
func (p *topUpTestProvider) Refund(context.Context, billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	return billing.ProviderObservation{}, billing.ErrReconciliationRequired
}
func (p *topUpTestProvider) QueryRefund(context.Context, billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	if p.refundObservation.EventID != "" {
		return p.refundObservation, nil
	}
	return billing.ProviderObservation{}, billing.ErrReconciliationRequired
}

func TestTopUpRefundHintBlocksSpendableCreditUntilConfirmed(t *testing.T) {
	r := commercialRepository(t)
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	wallet, _ := moneystore.New(r.db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	p := &topUpTestProvider{merchant: topUpMerchant(billing.PaymentAlipay)}
	if err := s.EnableWalletTopUps(r, wallet, p, nil, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, &topUpTestAuthorizer{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order, err := s.CreateWalletTopUpOrder(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: billing.PaymentAlipay, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "refund-first"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	paid := billing.ProviderObservation{Merchant: p.merchant, MerchantOrderID: a.MerchantOrderID, EventID: "payment", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: time.Now().UTC(), VerificationVersion: "v1"}
	hint := paid
	hint.EventID = "refund-hint"
	hint.Kind = "REFUND"
	hint.State = "REFUND_PENDING"
	hint.TotalMinor = 10000
	hint.RefundRequestID = "refund-1"
	for _, o := range []billing.ProviderObservation{hint, paid} {
		if err = s.RecordVerifiedPaymentObservation(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.ReconcileTopUpOrder(ctx, "org-1", order.OrderID); !errors.Is(err, billing.ErrReconciliationRequired) {
		t.Fatalf("unresolved refund: %v", err)
	}
	got, _ := s.ReadWallet(ctx, "org-1")
	if got.AvailableMinor != 0 {
		t.Fatal("credited despite known unresolved refund")
	}
	p.refundObservation = hint
	p.refundObservation.EventID = "refund-confirmed"
	p.refundObservation.State = "REFUNDED"
	if err = s.ReconcileTopUpOrder(ctx, "org-1", order.OrderID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ReadWallet(ctx, "org-1")
	if got.AvailableMinor != 0 || got.DebtMinor != 0 {
		t.Fatalf("full refund before credit: %+v", got)
	}
}

func TestTopUpBothChannelsCompleteOnlyAfterDurableExactPosting(t *testing.T) {
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		t.Run(string(channel), func(t *testing.T) {
			r := commercialRepository(t)
			if err := moneystore.AutoMigrate(r.db); err != nil {
				t.Fatal(err)
			}
			wallet, _ := moneystore.New(r.db)
			s, _ := billing.NewService(r, r, r, r, wallet, nil)
			p := &topUpTestProvider{merchant: topUpMerchant(channel)}
			auth := &topUpTestAuthorizer{}
			alipay, wechat := billing.TopUpProviderPort(nil), billing.TopUpProviderPort(nil)
			if channel == billing.PaymentAlipay {
				alipay = p
			} else {
				wechat = p
			}
			if err := s.EnableWalletTopUps(r, wallet, alipay, wechat, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, auth); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			order, err := s.CreateWalletTopUpOrder(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: channel, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "key-1"})
			if err != nil {
				t.Fatal(err)
			}
			a, _ := r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
			action, err := s.CheckoutTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version)
			if err != nil || !action.Matches(a) {
				t.Fatalf("checkout %+v %v", action, err)
			}
			a, _ = r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
			if _, err := s.CheckoutTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version); err != nil || p.checkouts != 1 {
				t.Fatalf("duplicate checkout: %d %v", p.checkouts, err)
			}
			paid := billing.ProviderObservation{Merchant: p.merchant, MerchantOrderID: a.MerchantOrderID, EventID: "paid-1", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: time.Now().UTC(), VerificationVersion: "v1"}
			if err := s.RecordVerifiedPaymentObservation(ctx, paid); err != nil {
				t.Fatal(err)
			}
			before, _ := s.ReadWallet(ctx, "org-1")
			if before.AvailableMinor != 0 {
				t.Fatal("ACK changed wallet before reconciliation")
			}
			auth.denied = true // verified original money must converge after revocation
			if err := s.ReconcileTopUpOrder(ctx, "org-1", order.OrderID); err != nil {
				t.Fatal(err)
			}
			completed, err := s.ReadOrder(ctx, "org-1", order.OrderID)
			if err != nil || completed.Status != billing.OrderFulfilled || completed.PaymentID == "" {
				t.Fatalf("completed %+v %v", completed, err)
			}
			after, _ := s.ReadWallet(ctx, "org-1")
			if after.AvailableMinor != 10000 {
				t.Fatalf("wallet %+v", after)
			}
			if err := s.ReconcileTopUpOrder(ctx, "org-1", order.OrderID); err != nil {
				t.Fatal(err)
			}
			after, _ = s.ReadWallet(ctx, "org-1")
			if after.AvailableMinor != 10000 {
				t.Fatal("duplicate credit")
			}
		})
	}
}

func TestTopUpLostNativeCheckoutNeverRedispatches(t *testing.T) {
	r := commercialRepository(t)
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	wallet, _ := moneystore.New(r.db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	p := &topUpTestProvider{merchant: topUpMerchant(billing.PaymentWeChat), lost: true}
	auth := &topUpTestAuthorizer{}
	if err := s.EnableWalletTopUps(r, wallet, nil, p, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, auth); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order, err := s.CreateWalletTopUpOrder(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: billing.PaymentWeChat, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "key-1"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	if _, err := s.CheckoutTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version); !errors.Is(err, billing.ErrReconciliationRequired) {
		t.Fatalf("lost response: %v", err)
	}
	a, _ = r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	if _, err := s.CheckoutTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version); !errors.Is(err, billing.ErrReconciliationRequired) || p.checkouts != 1 {
		t.Fatalf("redispatch %d %v", p.checkouts, err)
	}
}

func TestTopUpConfirmedClosureDoesNotRequireAnotherCloseCall(t *testing.T) {
	r := commercialRepository(t)
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	wallet, _ := moneystore.New(r.db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	p := &topUpTestProvider{merchant: topUpMerchant(billing.PaymentWeChat)}
	if err := s.EnableWalletTopUps(r, wallet, nil, p, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, &topUpTestAuthorizer{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order, err := s.CreateWalletTopUpOrder(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: billing.PaymentWeChat, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "closed"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	if _, err = s.CheckoutTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version); err != nil {
		t.Fatal(err)
	}
	a, _ = r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	p.observation = billing.ProviderObservation{Merchant: p.merchant, MerchantOrderID: a.MerchantOrderID, EventID: "closed", Kind: "PAYMENT", State: "CLOSED", Currency: "CNY", AmountMinor: 10000, VerificationVersion: "v1"}
	if err = s.CancelTopUp(ctx, "org-1", "admin-1", order.OrderID, a.Version); err != nil {
		t.Fatal(err)
	}
	a, _ = r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	if a.Phase != billing.TopUpClosedUnpaid {
		t.Fatalf("phase %s", a.Phase)
	}
}

func TestTopUpRefundRevokedBeforeDispatchReleasesPreparedHold(t *testing.T) {
	r := commercialRepository(t)
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	wallet, _ := moneystore.New(r.db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	p := &topUpTestProvider{merchant: topUpMerchant(billing.PaymentAlipay)}
	auth := &topUpTestAuthorizer{}
	if err := s.EnableWalletTopUps(r, wallet, p, nil, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, auth); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order, err := s.CreateWalletTopUpOrder(ctx, billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: billing.PaymentAlipay, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "refund"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	paid := billing.ProviderObservation{Merchant: p.merchant, MerchantOrderID: a.MerchantOrderID, EventID: "paid", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: time.Now().UTC(), VerificationVersion: "v1"}
	if err = s.RecordVerifiedPaymentObservation(ctx, paid); err != nil {
		t.Fatal(err)
	}
	if err = s.ReconcileTopUpOrder(ctx, "org-1", order.OrderID); err != nil {
		t.Fatal(err)
	}
	a, _ = r.ReadTopUpAttempt(ctx, "org-1", order.OrderID)
	auth.denyAt = auth.calls + 2
	refund, err := s.ApproveTopUpRefund(ctx, "platform-admin", order.OrderID, "refund-key", "requested", 6000, a.Version)
	if !errors.Is(err, billing.ErrAuthorizationRevoked) {
		t.Fatalf("revocation: %v", err)
	}
	snapshot, _ := s.ReadWallet(ctx, "org-1")
	if snapshot.AvailableMinor != 10000 || snapshot.ReservedMinor != 0 || refund.State != "RELEASED" || refund.Dispatched {
		t.Fatalf("hold retained: %+v %+v", snapshot, refund)
	}
}
