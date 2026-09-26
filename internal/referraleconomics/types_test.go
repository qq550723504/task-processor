package referraleconomics

import (
	"math"
	"testing"
)

func TestCommissionUsesMinorUnitsAndFixedTenPercent(t *testing.T) {
	value, err := CommissionForCashMinor(12345)
	if err != nil || value != 1234 {
		t.Fatalf("value=%d err=%v", value, err)
	}
}

func TestRefundCommissionUsesCumulativeFloorWithoutIntermediateOverflow(t *testing.T) {
	commission := int64(math.MaxInt64 / 10)
	netCash := int64(math.MaxInt64)
	adjustment, err := CommissionRefundAdjustment(commission, netCash, netCash/2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if adjustment < 0 || adjustment > 1 {
		t.Fatalf("adjustment=%d, want a nonnegative one-minor-unit step", adjustment)
	}
	refunded, err := CommissionForRefundedCashMinor(commission, netCash, netCash)
	if err != nil || refunded != commission {
		t.Fatalf("refunded=%d err=%v, want %d", refunded, err, commission)
	}
}

func TestRefundCommissionRejectsOverlappingRefundAmount(t *testing.T) {
	if _, err := CommissionRefundAdjustment(100, 1000, 900, 101); err != ErrInvalid {
		t.Fatalf("err=%v, want %v", err, ErrInvalid)
	}
}

func TestWithdrawalEligibilityRequiresMinimumAndSpendableAmount(t *testing.T) {
	base := RequestWithdrawal{Referrer: "person-1", Currency: CurrencyCNY, PayoutMethodID: "method-1", AmountMinor: MinimumWithdrawalMinor, Method: MethodAlipay, IdempotencyKey: "withdrawal-1"}
	if err := ValidateWithdrawalInput(base, MinimumWithdrawalMinor); err != nil {
		t.Fatal(err)
	}
	base.AmountMinor--
	if err := ValidateWithdrawalInput(base, MinimumWithdrawalMinor); err != ErrInvalid {
		t.Fatalf("err=%v", err)
	}
	base.AmountMinor = MinimumWithdrawalMinor
	if err := ValidateWithdrawalInput(base, MinimumWithdrawalMinor-1); err != ErrInsufficient {
		t.Fatalf("err=%v", err)
	}
}
