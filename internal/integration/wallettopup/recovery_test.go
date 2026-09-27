package wallettopup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recoveryAuth struct{}

func (recoveryAuth) AuthorizeTopUp(context.Context, string, string, bool) error { return nil }

// This exercises signed SDK responses through billing and the actual money owner.
func recoveryFixture(t *testing.T, channel billing.PaymentProvider, expiry time.Time, state string, tamper bool) (*billing.Service, *commercialstore.Repository, billing.TopUpPaymentAttempt, *int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "wallet.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = commercialstore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err = moneystore.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	r, _ := commercialstore.New(db)
	wallet, _ := moneystore.New(db)
	s, _ := billing.NewService(r, r, r, r, wallet, nil)
	merchant := billing.TopUpMerchant{Provider: channel, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: "PAGE_PAY"}
	if channel == billing.PaymentWeChat {
		merchant.Product = "NATIVE"
	}
	a, err := r.CreateTopUpAttempt(context.Background(), billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Provider: channel, Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "recovery"}, merchant, expiry.Add(-10*time.Minute), expiry)
	if err != nil {
		t.Fatal(err)
	}
	key, private, public := fixtureKeys(t)
	calls := new(int)
	c := providerHTTP()
	c.HttpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*calls++
		code := http.StatusOK
		var body string
		headers := http.Header{}
		if channel == billing.PaymentAlipay {
			body = `{"code":"10000","out_trade_no":"` + a.MerchantOrderID + `","trade_no":"trade-1","total_amount":"100.00","trade_status":"` + state + `","send_pay_date":"2026-09-27 12:00:00"}`
			if state == "NOT_EXIST" {
				body = `{"code":"40004","sub_code":"ACQ.TRADE_NOT_EXIST","msg":"Business Failed"}`
			}
			sig := signFixture(t, key, body)
			if tamper {
				body = strings.Replace(body, "40004", "40005", 1)
			}
			raw, _ := json.Marshal(map[string]any{"alipay_trade_query_response": json.RawMessage(body), "sign": sig})
			body = string(raw)
		} else {
			body = `{"appid":"app-1","mchid":"merchant-1","out_trade_no":"` + a.MerchantOrderID + `","trade_state":"` + state + `","trade_type":"NATIVE","transaction_id":"trade-1","success_time":"2026-09-27T12:00:00+08:00","amount":{"total":10000,"currency":"CNY"}}`
			if state == "NOT_EXIST" {
				code = http.StatusNotFound
				body = `{"code":"ORDER_NOT_EXIST","message":"not found"}`
			}
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			sig := signFixture(t, key, ts+"\nnonce\n"+body+"\n")
			headers = http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"nonce"}, "Wechatpay-Serial": {"PUB_KEY_ID_test"}, "Wechatpay-Signature": {sig}}
			if tamper {
				headers.Set("Wechatpay-Signature", "invalid")
			}
		}
		return &http.Response{StatusCode: code, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	var ali, wx billing.TopUpProviderPort
	if channel == billing.PaymentAlipay {
		p, e := NewAlipay(AlipayConfig{Merchant: merchant, NewPayments: true, PrivateKey: private, PublicKey: public, NotifyURL: "https://merchant.example/notify", ReturnURL: "https://merchant.example/orders"})
		if e != nil {
			t.Fatal(e)
		}
		p.client.SetHttpClient(c)
		ali = p
	} else {
		p, e := NewWeChat(WeChatConfig{Merchant: merchant, NewPayments: true, PrivateKey: private, PublicKey: public, PublicKeyID: "PUB_KEY_ID_test", SerialNumber: "serial", APIv3Key: strings.Repeat("a", 32), NotifyURL: "https://merchant.example/notify"})
		if e != nil {
			t.Fatal(e)
		}
		p.client.SetHttpClient(c)
		wx = p
	}
	protection, err := NewPayloadProtection([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableWalletTopUps(r, wallet, ali, wx, billing.TopUpAmountPolicy{}, protection, recoveryAuth{}); err != nil {
		t.Fatal(err)
	}
	return s, r, a, calls
}

func TestRefundedPaymentQueryNeverCreatesSpendableCredit(t *testing.T) {
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		t.Run(string(channel), func(t *testing.T) {
			state := "TRADE_CLOSED"
			if channel == billing.PaymentWeChat {
				state = "REFUND"
			}
			s, r, a, _ := recoveryFixture(t, channel, time.Now().UTC().Add(time.Minute), state, false)
			ctx := context.Background()
			admitted := a.CreatedAt
			a.CheckoutAdmittedAt = &admitted
			a, err := r.SaveTopUpAttempt(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); !errors.Is(err, billing.ErrReconciliationRequired) {
				t.Fatalf("refunded query: %v", err)
			}
			w, _ := s.ReadWallet(ctx, a.OrganizationID)
			if w.AvailableMinor != 0 || w.LifetimeTopUpMinor != 0 {
				t.Fatalf("refunded money credited: %+v", w)
			}
			// A delayed payment callback must not erase already-known refund uncertainty.
			paid := billing.ProviderObservation{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, EventID: "paid-callback", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: aliTime("2026-09-27 12:00:00"), VerificationVersion: "fixture-v1"}
			if err = s.RecordVerifiedPaymentObservation(ctx, paid); err != nil {
				t.Fatal(err)
			}
			if err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); !errors.Is(err, billing.ErrReconciliationRequired) {
				t.Fatalf("late callback lost refund hint: %v", err)
			}
			refund := paid
			refund.EventID = "verified-full-refund"
			refund.Kind = "REFUND"
			refund.State = "REFUNDED"
			refund.TotalMinor = 10000
			refund.RefundRequestID = "original-refund"
			if err = s.RecordVerifiedPaymentObservation(ctx, refund); err != nil {
				t.Fatal(err)
			}
			if err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
				t.Fatalf("verified full reversal: %v", err)
			}
			w, _ = s.ReadWallet(ctx, a.OrganizationID)
			if w.AvailableMinor != 0 || w.DebtMinor != 0 {
				t.Fatalf("full reversal not atomic: %+v", w)
			}
		})
	}
}

func TestPaymentAbsentClosesOnlyWithVerifiedEvidenceAfterOriginalDeadline(t *testing.T) {
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		for _, scenario := range []struct {
			name            string
			expired, tamper bool
		}{{"before_deadline", false, false}, {"expired", true, false}, {"untrusted_absence", true, true}} {
			t.Run(string(channel)+"/"+scenario.name, func(t *testing.T) {
				expiry := time.Now().UTC().Add(time.Minute)
				if scenario.expired {
					expiry = expiry.Add(-2 * time.Minute)
				}
				s, r, a, _ := recoveryFixture(t, channel, expiry, "NOT_EXIST", scenario.tamper)
				ctx := context.Background()
				admitted := a.CreatedAt
				a.CheckoutAdmittedAt = &admitted
				a, err := r.SaveTopUpAttempt(ctx, a)
				if err != nil {
					t.Fatal(err)
				}
				err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID)
				a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
				if scenario.expired && !scenario.tamper {
					if err != nil || a.Phase != billing.TopUpClosedUnpaid {
						t.Fatalf("expired absence did not close: %s %v", a.Phase, err)
					}
				} else if !errors.Is(err, billing.ErrReconciliationRequired) || a.Phase == billing.TopUpClosedUnpaid {
					t.Fatalf("premature or untrusted closure: %s %v", a.Phase, err)
				}
			})
		}
	}
}

func TestNativeLateCheckoutClosesWithoutDispatch(t *testing.T) {
	s, r, a, calls := recoveryFixture(t, billing.PaymentWeChat, time.Now().UTC().Add(45*time.Second), "NOT_EXIST", false)
	_, err := s.CheckoutTopUp(context.Background(), a.OrganizationID, a.ActorID, a.OrderID, a.Version)
	a, _ = r.ReadTopUpAttempt(context.Background(), a.OrganizationID, a.OrderID)
	if !errors.Is(err, billing.ErrPaymentMethodUnavailable) || a.Phase != billing.TopUpClosedUnpaid || *calls != 0 {
		t.Fatalf("late Native checkout stuck or dispatched: %s %v requests=%d", a.Phase, err, *calls)
	}
}

func TestUnvisitedAlipayCheckoutSurvivesVerifiedAbsenceBeforeDeadline(t *testing.T) {
	s, r, a, calls := recoveryFixture(t, billing.PaymentAlipay, time.Now().UTC().Add(2*time.Minute), "NOT_EXIST", false)
	ctx := context.Background()
	original, err := s.CheckoutTopUp(ctx, a.OrganizationID, a.ActorID, a.OrderID, a.Version)
	if err != nil || original.Kind != "REDIRECT" || *calls != 0 {
		t.Fatalf("Page Pay URL creation: %+v %v requests=%d", original, err, *calls)
	}
	if err = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
		t.Fatalf("unvisited URL became unusable: %v", err)
	}
	a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
	replayed, err := s.CheckoutTopUp(ctx, a.OrganizationID, a.ActorID, a.OrderID, a.Version)
	if err != nil || a.Phase != billing.TopUpAwaitingPayment || original != replayed || *calls != 1 {
		t.Fatalf("original URL was not preserved: %s %v requests=%d", a.Phase, err, *calls)
	}
}
