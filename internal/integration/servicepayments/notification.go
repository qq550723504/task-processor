package servicepayments

import (
	wechat "github.com/go-pay/gopay/wechat/v3"
	"net/http"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"task-processor/internal/ledger/money"
)

func (p *WeChat) VerifyServiceNotification(r *http.Request) (billing.ServicePaymentObservation, error) {
	var empty billing.ServicePaymentObservation
	id, event, plain, err := paymentsecurity.VerifiedWeChatNotification(r, p.config.PublicKeyID, p.publicKey, p.config.APIv3Key, p.now())
	if err != nil || event != "TRANSACTION.SUCCESS" {
		return empty, billing.ErrInvalid
	}
	var v wechat.PartnerQueryOrder
	if paymentsecurity.StrictJSON(plain, &v) != nil || v.TradeState != "SUCCESS" || v.Amount == nil || v.Amount.Total <= 0 || v.SubMchid == "" || v.SubMchid == p.Profile().PlatformMerchantID || v.OutTradeNo == "" || len(v.OutTradeNo) > 32 {
		return empty, billing.ErrInvalid
	}
	// This only normalizes a signed channel fact. The inbox consumer matches
	// every field against the immutable original order before persisting it.
	o := billing.ServicePurchaseOrder{Profile: p.Profile(), Source: billing.ServicePurchaseCommand{ProviderMerchantID: v.SubMchid, AmountMinor: int64(v.Amount.Total)}, TradeNo: v.OutTradeNo}
	o.Source.Allocation.Basis = money.ServiceAllocationChannelNetFloorV2
	obs, err := p.paymentResult(o, v, string(plain))
	if err != nil {
		return empty, err
	}
	obs.EventID = id
	return obs, nil
}
