package money

import "sort"

const ServiceAllocationChannelNetFloorV2 = "CHANNEL_SETTLEMENT_NET_FLOOR_V2"

type ServiceVoucher struct {
	ID, FundingType string
	AmountMinor     int64
}
type ServicePaymentAmounts struct {
	PayerMinor int64
	Vouchers   []ServiceVoucher
}

func (a ServicePaymentAmounts) Normalize() ServicePaymentAmounts {
	a.Vouchers = append([]ServiceVoucher(nil), a.Vouchers...)
	sort.Slice(a.Vouchers, func(i, j int) bool { return a.Vouchers[i].ID < a.Vouchers[j].ID })
	return a
}
func (a ServicePaymentAmounts) Validate(total int64) error {
	if total <= 0 || a.PayerMinor < 0 || a.PayerMinor > total || len(a.Vouchers) > 100 {
		return ErrInvalid
	}
	remaining := total - a.PayerMinor
	seen := map[string]bool{}
	for _, v := range a.Vouchers {
		if !isCanonicalWalletIdentifier(v.ID) || seen[v.ID] || v.FundingType != "CASH" && v.FundingType != "NOCASH" || v.AmountMinor <= 0 || v.AmountMinor > remaining {
			return ErrInvalid
		}
		seen[v.ID] = true
		remaining -= v.AmountMinor
	}
	if remaining != 0 {
		return ErrInvalid
	}
	return nil
}
func (a ServicePaymentAmounts) SettlementMinor() int64 {
	n := a.PayerMinor
	for _, v := range a.Vouchers {
		if v.FundingType == "CASH" {
			n += v.AmountMinor
		}
	}
	return n
}

type ServiceVoucherRefund struct {
	ID, RefundType, FundingType      string
	OriginalAmountMinor, RefundMinor int64
}
type ServiceRefundAmounts struct {
	PayerMinor, DiscountMinor int64
	Vouchers                  []ServiceVoucherRefund
}

func (a ServiceRefundAmounts) Normalize() ServiceRefundAmounts {
	a.Vouchers = append([]ServiceVoucherRefund(nil), a.Vouchers...)
	sort.Slice(a.Vouchers, func(i, j int) bool { return a.Vouchers[i].ID < a.Vouchers[j].ID })
	return a
}
func (a ServiceRefundAmounts) Validate(payment ServicePaymentAmounts, nominal int64) error {
	if nominal <= 0 || a.PayerMinor < 0 || a.PayerMinor > nominal || a.PayerMinor > payment.PayerMinor || a.DiscountMinor < 0 || a.DiscountMinor != nominal-a.PayerMinor || len(a.Vouchers) > 100 {
		return ErrInvalid
	}
	original := map[string]ServiceVoucher{}
	for _, v := range payment.Vouchers {
		original[v.ID] = v
	}
	remaining := a.DiscountMinor
	seen := map[string]bool{}
	for _, v := range a.Vouchers {
		o, ok := original[v.ID]
		if !ok || seen[v.ID] || v.RefundType != "COUPON" && v.RefundType != "DISCOUNT" || v.FundingType != o.FundingType || v.OriginalAmountMinor != o.AmountMinor || v.RefundMinor < 0 || v.RefundMinor > o.AmountMinor || v.RefundMinor > remaining {
			return ErrInvalid
		}
		seen[v.ID] = true
		remaining -= v.RefundMinor
	}
	if remaining != 0 {
		return ErrInvalid
	}
	return nil
}
func (a ServiceRefundAmounts) SettlementMinor() int64 {
	n := a.PayerMinor
	for _, v := range a.Vouchers {
		if v.FundingType == "CASH" {
			n += v.RefundMinor
		}
	}
	return n
}
