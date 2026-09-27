package commercialbilling

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercial/billing"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	moneystore "task-processor/internal/integration/persistence/money"
	"task-processor/internal/ledger/money"

	"github.com/gin-gonic/gin"
)

func reviewTopUp(t *testing.T, channel billing.PaymentProvider) (*Repository, *billing.Service, *moneystore.Repository, *topUpTestProvider, billing.TopUpPaymentAttempt) {
	t.Helper()
	r := commercialRepository(t)
	if err := moneystore.AutoMigrate(r.db); err != nil {
		t.Fatal(err)
	}
	wallet, _ := moneystore.New(r.db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	p := &topUpTestProvider{merchant: topUpMerchant(channel)}
	var ali, wx billing.TopUpProviderPort
	if channel == billing.PaymentAlipay {
		ali = p
	} else {
		wx = p
	}
	if err := s.EnableWalletTopUps(r, wallet, ali, wx, billing.TopUpAmountPolicy{MinMinor: 100, MaxMinor: 10000, QuickAmounts: []int64{10000}, PaymentWindow: 10 * time.Minute}, topUpTestProtection{}, &topUpTestAuthorizer{}); err != nil {
		t.Fatal(err)
	}
	order, err := s.CreateWalletTopUpOrder(context.Background(), billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: channel, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "review"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := r.ReadTopUpAttempt(context.Background(), "org-1", order.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	return r, s, wallet, p, a
}

func TestTopUpDetailHTTPContainsPaymentAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		t.Run(string(channel), func(t *testing.T) {
			_, s, _, _, a := reviewTopUp(t, channel)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			r := httptest.NewRequest("GET", "/api/v1/commercial/orders/"+a.OrderID, nil)
			c.Request = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: a.ActorID, TenantID: a.OrganizationID, EffectiveOrganizationID: a.OrganizationID}))
			c.Params = gin.Params{{Key: "order_id", Value: a.OrderID}}
			billinghttp.NewHandler(s).Order(c)
			var response struct {
				TopUp *struct {
					Provider  billing.PaymentProvider `json:"provider"`
					AttemptID string                  `json:"attempt_id"`
					Phase     billing.TopUpPhase      `json:"phase"`
				} `json:"top_up"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != 200 || response.TopUp == nil || response.TopUp.Provider != channel || response.TopUp.AttemptID != a.AttemptID || response.TopUp.Phase != billing.TopUpCreated {
				t.Fatalf("payment detail missing: %d %s (%v)", w.Code, w.Body.String(), err)
			}
		})
	}
}

func TestTopUpNormalRecoveryPreservesCheckout(t *testing.T) {
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		t.Run(string(channel), func(t *testing.T) {
			r, s, _, p, a := reviewTopUp(t, channel)
			ctx := context.Background()
			if err := s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
				t.Fatalf("created recovery: %v", err)
			}
			a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
			if a.Phase != billing.TopUpCreated {
				t.Fatalf("created became %s", a.Phase)
			}
			first, err := s.CheckoutTopUp(ctx, a.OrganizationID, a.ActorID, a.OrderID, a.Version)
			if err != nil {
				t.Fatal(err)
			}
			p.observation = billing.ProviderObservation{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, EventID: "unpaid", Kind: "PAYMENT", State: "UNPAID", Currency: "CNY", VerificationVersion: "v1"}
			if err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
				t.Fatalf("unpaid recovery: %v", err)
			}
			a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
			replayed, err := s.CheckoutTopUp(ctx, a.OrganizationID, a.ActorID, a.OrderID, a.Version)
			if err != nil || a.Phase != billing.TopUpAwaitingPayment || first != replayed || p.checkouts != 1 {
				t.Fatalf("pending checkout lost: %+v %v calls=%d", a, err, p.checkouts)
			}
		})
	}
}

type refundAdmissionRace struct {
	money.ProviderTopUpOwner
	reversal              money.OrganizationWalletReversal
	loseAdmissionResponse bool
}

func (m refundAdmissionRace) AdmitTopUpRefund(ctx context.Context, input money.TopUpRefundInput) (money.TopUpRefundHold, error) {
	if m.loseAdmissionResponse {
		h, err := m.ProviderTopUpOwner.AdmitTopUpRefund(ctx, input)
		if err != nil {
			return h, err
		}
		return h, errors.New("admission response lost")
	}
	if _, err := m.AcceptProviderTopUpReversal(ctx, m.reversal); err != nil {
		return money.TopUpRefundHold{}, err
	}
	return m.ProviderTopUpOwner.AdmitTopUpRefund(ctx, input)
}

func TestTopUpRefundAdmissionFailureReleasesOnlyNeverAdmittedHold(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget_changed", true: "unknown_admission"}[lost], func(t *testing.T) {
			r, s, wallet, p, a := reviewTopUp(t, billing.PaymentAlipay)
			ctx := context.Background()
			paid := billing.ProviderObservation{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, EventID: "paid", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: time.Now().UTC(), VerificationVersion: "v1"}
			if err := s.RecordVerifiedPaymentObservation(ctx, paid); err != nil {
				t.Fatal(err)
			}
			if err := s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
				t.Fatal(err)
			}
			a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
			owner := refundAdmissionRace{ProviderTopUpOwner: wallet, loseAdmissionResponse: lost, reversal: money.OrganizationWalletReversal{ReversalID: "external-race", PaymentID: a.PaymentID, Kind: money.WalletReversalChargeback, OrganizationID: a.OrganizationID, CommercialOrderID: a.OrderID, Currency: "CNY", AmountMinor: 5000, OccurredAt: time.Now().UTC(), ProviderReference: "external-race"}}
			if err := s.EnableWalletTopUps(r, owner, p, nil, billing.TopUpAmountPolicy{}, topUpTestProtection{}, &topUpTestAuthorizer{}); err != nil {
				t.Fatal(err)
			}
			refund, err := s.ApproveTopUpRefund(ctx, "platform-admin", a.OrderID, "refund-key", "requested", 6000, a.Version)
			if err == nil {
				t.Fatal("expected admission failure")
			}
			w, err := s.ReadWallet(ctx, a.OrganizationID)
			if err != nil {
				t.Fatal(err)
			}
			if lost {
				if refund.State == "RELEASED" || w.ReservedMinor != 6000 {
					t.Fatalf("unknown admission released: %+v %+v", w, refund)
				}
			} else if refund.State != "RELEASED" || refund.Dispatched || w.AvailableMinor != 5000 || w.ReservedMinor != 0 || w.DebtMinor != 0 {
				t.Fatalf("unadmitted hold failed to converge: %+v %+v", w, refund)
			}
		})
	}
}
