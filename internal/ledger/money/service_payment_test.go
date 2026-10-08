package money

import (
	"testing"
	"time"
)

func TestServicePaymentIsExplicitlyNonCommissionable(t *testing.T) {
	p := PaymentSettlement{PaymentID: "service-payment", PaymentPurpose: "SERVICE_PURCHASE", CommissionTreatment: CommissionNonCommissionable, PayerBinding: "ORGANIZATION_SERVICE_BUYER", Currency: WalletCurrencyCNY, GrossAmountMinor: 10001, Status: PaymentSettled, SettledAt: time.Now(), ProviderReference: "wechat-original-transaction", Version: 1}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid third-party service payment was rejected: %v", err)
	}
	for _, mutate := range []func(*PaymentSettlement){
		func(v *PaymentSettlement) { v.CommissionableAmountMinor = 1 },
		func(v *PaymentSettlement) { v.PayerUserID = "referral-user" },
		func(v *PaymentSettlement) { v.CommissionTreatment = "" },
		func(v *PaymentSettlement) { v.PayerBinding = PayerUnattributedExternal },
		func(v *PaymentSettlement) { v.DiscountAmountMinor = 1 },
	} {
		v := p
		mutate(&v)
		if v.Validate() == nil {
			t.Fatal("invalid service payment can enter a commissionable or unrelated payer path")
		}
	}
}
