package money

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestProviderTopUpAcceptsExplicitUnattributedNonCommissionablePayment(t *testing.T) {
	var payment PaymentSettlement
	err := json.Unmarshal([]byte(`{"PaymentID":"topup-1","PaymentPurpose":"WALLET_TOP_UP","CommissionTreatment":"NON_COMMISSIONABLE","PayerBinding":"UNATTRIBUTED_EXTERNAL","Currency":"CNY","GrossAmountMinor":10000,"Status":"SETTLED","SettledAt":"2026-09-27T00:00:00Z","ProviderReference":"provider-1","Version":1}`), &payment)
	if err != nil {
		t.Fatal(err)
	}
	if err := payment.Validate(); err != nil {
		t.Fatalf("explicit non-commissionable top-up: %v", err)
	}
	for _, change := range []func(*PaymentSettlement){
		func(p *PaymentSettlement) { p.PayerUserID = "invented-user" },
		func(p *PaymentSettlement) { p.CommissionableAmountMinor = 1 },
		func(p *PaymentSettlement) { p.DiscountAmountMinor = 1 },
		func(p *PaymentSettlement) { p.Currency = "USD" },
	} {
		invalid := payment
		change(&invalid)
		if !errors.Is(invalid.Validate(), ErrInvalid) {
			t.Fatalf("accepted invalid top-up: %+v", invalid)
		}
	}
}

func TestOrdinaryPaymentStillRequiresPayerAndPositiveCommission(t *testing.T) {
	var payment PaymentSettlement
	if err := json.Unmarshal([]byte(`{"PaymentID":"ordinary-1","Currency":"CNY","GrossAmountMinor":10000,"Status":"SETTLED","SettledAt":"2026-09-27T00:00:00Z","ProviderReference":"provider-1","Version":1}`), &payment); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(payment.Validate(), ErrInvalid) {
		t.Fatal("unclassified payment accepted without payer/commission")
	}
	payment.PayerUserID, payment.CommissionableAmountMinor = "user-1", 10000
	if err := payment.Validate(); err != nil {
		t.Fatal(err)
	}
}
