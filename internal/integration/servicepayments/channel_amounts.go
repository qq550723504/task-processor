package servicepayments

import (
	wechat "github.com/go-pay/gopay/wechat/v3"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"task-processor/internal/ledger/money"
)

// Required zero-valued channel fields must be present in the signed body.
// SDK int defaults cannot distinguish an all-voucher payment from omission.
func requiredPaymentAmounts(raw string) error {
	var body struct {
		Amount *struct {
			Total      *int64 `json:"total"`
			PayerTotal *int64 `json:"payer_total"`
			Currency   string `json:"currency"`
		} `json:"amount"`
		Promotion []struct {
			ID     string `json:"coupon_id"`
			Type   string `json:"type"`
			Amount *int64 `json:"amount"`
		} `json:"promotion_detail"`
	}
	if paymentsecurity.StrictJSON([]byte(raw), &body) != nil || body.Amount == nil || body.Amount.Total == nil || body.Amount.PayerTotal == nil || body.Amount.Currency != "CNY" || len(body.Promotion) > 100 {
		return billing.ErrConflict
	}
	for _, v := range body.Promotion {
		if v.ID == "" || v.Type == "" || v.Amount == nil {
			return billing.ErrConflict
		}
	}
	return nil
}
func requiredRefundAmounts(raw string) error {
	var body struct {
		Amount *struct {
			Refund         *int64 `json:"refund"`
			PayerRefund    *int64 `json:"payer_refund"`
			DiscountRefund *int64 `json:"discount_refund"`
			Currency       string `json:"currency"`
		} `json:"amount"`
		Promotion []struct {
			ID     string `json:"promotion_id"`
			Type   string `json:"type"`
			Amount *int64 `json:"amount"`
			Refund *int64 `json:"refund_amount"`
		} `json:"promotion_detail"`
	}
	if paymentsecurity.StrictJSON([]byte(raw), &body) != nil || body.Amount == nil || body.Amount.Refund == nil || body.Amount.PayerRefund == nil || body.Amount.DiscountRefund == nil || body.Amount.Currency != "CNY" || len(body.Promotion) > 100 {
		return billing.ErrConflict
	}
	for _, v := range body.Promotion {
		if v.ID == "" || v.Type == "" || v.Amount == nil || v.Refund == nil {
			return billing.ErrConflict
		}
	}
	return nil
}
func channelPaymentAmounts(v wechat.PartnerQueryOrder) (money.ServicePaymentAmounts, error) {
	var a money.ServicePaymentAmounts
	if v.Amount == nil || v.Amount.Currency != "CNY" || v.Amount.PayerCurrency != "" && v.Amount.PayerCurrency != "CNY" || len(v.PromotionDetail) > 100 {
		return a, billing.ErrConflict
	}
	a.PayerMinor = int64(v.Amount.PayerTotal)
	for _, p := range v.PromotionDetail {
		if p == nil || p.Currency != "" && p.Currency != "CNY" {
			return money.ServicePaymentAmounts{}, billing.ErrConflict
		}
		a.Vouchers = append(a.Vouchers, money.ServiceVoucher{ID: p.CouponId, FundingType: p.Type, AmountMinor: int64(p.Amount)})
	}
	if a.Validate(int64(v.Amount.Total)) != nil {
		return money.ServicePaymentAmounts{}, billing.ErrConflict
	}
	return a.Normalize(), nil
}
func channelRefundAmounts(payment money.ServicePaymentAmounts, v wechat.EcommerceRefundQuery) (money.ServiceRefundAmounts, error) {
	var a money.ServiceRefundAmounts
	if v.Amount == nil || v.Amount.Currency != "CNY" || len(v.PromotionDetail) > 100 {
		return a, billing.ErrConflict
	}
	a.PayerMinor = int64(v.Amount.PayerRefund)
	a.DiscountMinor = int64(v.Amount.DiscountRefund)
	original := map[string]money.ServiceVoucher{}
	for _, p := range payment.Vouchers {
		original[p.ID] = p
	}
	for _, p := range v.PromotionDetail {
		if p == nil {
			return money.ServiceRefundAmounts{}, billing.ErrConflict
		}
		o, ok := original[p.PromotionId]
		if !ok {
			return money.ServiceRefundAmounts{}, billing.ErrConflict
		}
		a.Vouchers = append(a.Vouchers, money.ServiceVoucherRefund{ID: p.PromotionId, RefundType: p.Type, FundingType: o.FundingType, OriginalAmountMinor: int64(p.Amount), RefundMinor: int64(p.RefundAmount)})
	}
	if a.Validate(payment, int64(v.Amount.Refund)) != nil {
		return money.ServiceRefundAmounts{}, billing.ErrConflict
	}
	return a.Normalize(), nil
}
