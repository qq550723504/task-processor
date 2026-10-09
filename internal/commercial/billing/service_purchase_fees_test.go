package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/ledger/money"
	"testing"
)

type financialFundsFixture struct {
	money.ServiceFundsStore
	view  money.ServiceFundsView
	order string
}

func (f *financialFundsFixture) ReadServiceFunds(_ context.Context, order string) (money.ServiceFundsView, error) {
	f.order = order
	return f.view, nil
}

type financialOrderFixture struct {
	ServicePurchaseStore
	order ServicePurchaseOrder
}

func (f financialOrderFixture) ReadServicePurchase(context.Context, string) (ServicePurchaseOrder, error) {
	return f.order, nil
}

type financialFeeFixture struct {
	ServicePurchaseProvider
	profile ServiceMerchantProfile
}

func (f financialFeeFixture) Profile() ServiceMerchantProfile { return f.profile }
func (f financialFeeFixture) QueryServiceFees(context.Context, ServicePurchaseOrder, string) ([]money.ServiceChannelFee, error) {
	return nil, nil
}

func TestServiceFinancialFactsPreserveCanonicalChargebacks(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		refunded, chargedBack, platform, provider int64
	}{
		{"none", 0, 0, 10, 91},
		{"partial", 0, 20, 8, 73},
		{"full", 0, 101, 0, 0},
		{"refund-and-chargeback", 2, 20, 7, 72},
	} {
		for _, refresh := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/refresh=%t", tc.name, refresh), func(t *testing.T) {
				funds := &financialFundsFixture{view: money.ServiceFundsView{OrderID: "original-order", GrossMinor: 101, RefundedMinor: tc.refunded, ChargedBackMinor: tc.chargedBack, PlatformMinor: tc.platform, ProviderMinor: tc.provider}}
				profile := ServiceMerchantProfile{Version: "original-profile"}
				s := &ServicePurchases{funds: funds, store: financialOrderFixture{order: ServicePurchaseOrder{Profile: profile, Payment: &ServicePaymentObservation{}, PaymentReceiptID: "original-receipt"}}, provider: financialFeeFixture{profile: profile}}
				var view ServiceFinancialView
				var err error
				if refresh {
					view, err = s.RefreshChannelFees(context.Background(), "original-order", "2026-10-08")
				} else {
					view, err = s.ReadFinancialFacts(context.Background(), "original-order")
				}
				if err != nil || funds.order != "original-order" {
					t.Fatalf("original funds read: order=%q err=%v", funds.order, err)
				}
				body, err := json.Marshal(view)
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]any
				if err = json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]int64{"grossMinor": 101, "refundedMinor": tc.refunded, "chargedBackMinor": tc.chargedBack, "platformMinor": tc.platform, "providerMinor": tc.provider} {
					if got[key] != fmt.Sprint(want) {
						t.Errorf("%s=%v, want exact string %d; JSON=%s", key, got[key], want, body)
					}
				}
			})
		}
	}
}
