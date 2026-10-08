// Package servicepayments adapts platform-receipt WeChat facts without wallet semantics.
package servicepayments

import (
	"context"
	"crypto/rsa"
	"github.com/go-pay/crypto/xpem"
	"github.com/go-pay/gopay"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"task-processor/internal/ledger/money"
	"time"
)

type WeChatConfig struct {
	Profile                                                               billing.ServiceMerchantProfile
	NewPayments, ProductQualified, PlatformPaysFees                       bool
	PrivateKey, SerialNumber, APIv3Key, PublicKeyID, PublicKey, NotifyURL string
}
type WeChat struct {
	config    WeChatConfig
	client    *wechat.ClientV3
	publicKey *rsa.PublicKey
	now       func() time.Time
}

func NewWeChat(cfg WeChatConfig) (*WeChat, error) {
	u, err := url.Parse(cfg.NotifyURL)
	if cfg.Profile.Validate() != nil || cfg.Profile.Environment != "PRODUCTION" || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || !strings.HasPrefix(cfg.PublicKeyID, "PUB_KEY_ID_") || cfg.SerialNumber == "" || len(cfg.APIv3Key) != 32 {
		return nil, billing.ErrInvalid
	}
	key, err := xpem.DecodePublicKey([]byte(cfg.PublicKey))
	if err != nil || key == nil || key.N.BitLen() < 2048 {
		return nil, billing.ErrInvalid
	}
	client, err := wechat.NewClientV3(cfg.Profile.PlatformMerchantID, cfg.SerialNumber, cfg.APIv3Key, cfg.PrivateKey)
	if err != nil {
		return nil, billing.ErrInvalid
	}
	if err := client.AutoVerifySignByPublicKey([]byte(cfg.PublicKey), cfg.PublicKeyID); err != nil {
		return nil, billing.ErrInvalid
	}
	client.SetHttpClient(paymentsecurity.HTTP())
	client.SetBodySize(1)
	client.SetLogger(paymentsecurity.SilentLogger{})
	return &WeChat{config: cfg, client: client, publicKey: key, now: time.Now}, nil
}
func (p *WeChat) Profile() billing.ServiceMerchantProfile { return p.config.Profile }
func (p *WeChat) NewPaymentsEnabled() bool {
	return p.config.NewPayments && p.config.ProductQualified && p.config.PlatformPaysFees
}
func (p *WeChat) matches(o billing.ServicePurchaseOrder) bool {
	return o.Profile == p.Profile() && o.Source.ProviderMerchantID != "" && o.Source.ProviderMerchantID != p.Profile().PlatformMerchantID
}
func (p *WeChat) CreateServiceCheckout(ctx context.Context, o billing.ServicePurchaseOrder) (string, error) {
	if !p.NewPaymentsEnabled() || !p.matches(o) || !o.PaymentDispatched || o.CancelRequested || o.ExpiresAt.Sub(p.now()) < 75*time.Second {
		return "", billing.ErrPaymentMethodUnavailable
	}
	rsp, err := p.client.V3PartnerTransactionNative(ctx, gopay.BodyMap{"sp_appid": p.Profile().AppID, "sp_mchid": p.Profile().PlatformMerchantID, "sub_mchid": o.Source.ProviderMerchantID, "description": "生态服务购买", "out_trade_no": o.TradeNo, "time_expire": o.ExpiresAt.Format(time.RFC3339), "notify_url": p.config.NotifyURL, "amount": gopay.BodyMap{"total": o.Source.AmountMinor, "currency": "CNY"}, "settle_info": gopay.BodyMap{"profit_sharing": true}})
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
		return "", billing.ErrReconciliationRequired
	}
	u, err := url.Parse(rsp.Response.CodeUrl)
	if err != nil || u.Scheme != "weixin" || u.Host != "wxpay" || u.Path != "/bizpayurl" || u.User != nil || u.Fragment != "" || u.Query().Get("pr") == "" {
		return "", billing.ErrReconciliationRequired
	}
	return rsp.Response.CodeUrl, nil
}
func (p *WeChat) QueryServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	var empty billing.ServicePaymentObservation
	if !p.matches(o) {
		return empty, billing.ErrConflict
	}
	rsp, err := p.client.V3PartnerQueryOrder(ctx, wechat.OutTradeNo, o.TradeNo, gopay.BodyMap{"sp_mchid": p.Profile().PlatformMerchantID, "sub_mchid": o.Source.ProviderMerchantID})
	if err == nil && rsp != nil && rsp.Code == http.StatusNotFound && rsp.ErrResponse.Code == "ORDER_NOT_EXIST" && p.verifiedError(rsp.SignInfo, "ORDER_NOT_EXIST") {
		state := "UNPAID"
		if !p.now().Before(o.ExpiresAt) {
			state = "CLOSED"
		}
		return p.paymentObservation(o, state, "", 0, time.Time{}), nil
	}
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
		return empty, billing.ErrReconciliationRequired
	}
	return p.paymentResult(o, *rsp.Response)
}
func (p *WeChat) paymentObservation(o billing.ServicePurchaseOrder, state, tx string, amount int64, at time.Time) billing.ServicePaymentObservation {
	v := billing.ServicePaymentObservation{ProfileVersion: p.Profile().Version, PlatformMerchantID: p.Profile().PlatformMerchantID, AppID: p.Profile().AppID, ProviderMerchantID: o.Source.ProviderMerchantID, TradeNo: o.TradeNo, TransactionID: tx, Currency: "CNY", State: state, AmountMinor: amount, OccurredAt: money.NormalizeTimestamp(at), VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}
	v.EventID = "service-payment:" + money.ServiceFingerprint(v)
	return v
}
func (p *WeChat) paymentResult(o billing.ServicePurchaseOrder, v wechat.PartnerQueryOrder) (billing.ServicePaymentObservation, error) {
	var empty billing.ServicePaymentObservation
	if v.SpAppid != p.Profile().AppID || v.SpMchid != p.Profile().PlatformMerchantID || v.SubMchid != o.Source.ProviderMerchantID || v.OutTradeNo != o.TradeNo || v.TradeType != "" && v.TradeType != "NATIVE" {
		return empty, billing.ErrConflict
	}
	state := ""
	switch v.TradeState {
	case "SUCCESS":
		state = "PAID"
	case "REFUND":
		state = "PAID_REFUND_UNKNOWN"
	case "NOTPAY", "USERPAYING":
		state = "UNPAID"
	case "CLOSED":
		state = "CLOSED"
	default:
		return empty, billing.ErrReconciliationRequired
	}
	var amount int64
	var at time.Time
	if state == "PAID" || state == "PAID_REFUND_UNKNOWN" {
		if v.TradeType != "NATIVE" || v.Amount == nil || v.Amount.Currency != "CNY" || int64(v.Amount.Total) != o.Source.AmountMinor || v.Amount.PayerTotal != v.Amount.Total || len(v.PromotionDetail) > 0 {
			return empty, billing.ErrConflict
		}
		amount = int64(v.Amount.Total)
		var err error
		at, err = time.Parse(time.RFC3339, v.SuccessTime)
		if err != nil {
			return empty, billing.ErrConflict
		}
	}
	obs := p.paymentObservation(o, state, v.TransactionId, amount, at)
	if !obs.Matches(o) {
		return empty, billing.ErrConflict
	}
	return obs, nil
}
func (p *WeChat) CloseServicePayment(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePaymentObservation, error) {
	if !p.matches(o) || !o.CancelRequested {
		return billing.ServicePaymentObservation{}, billing.ErrConflict
	}
	rsp, err := p.client.V3PartnerCloseOrder(ctx, o.TradeNo, gopay.BodyMap{"sp_mchid": p.Profile().PlatformMerchantID, "sub_mchid": o.Source.ProviderMerchantID})
	if err != nil || rsp == nil || rsp.Code != 0 {
		return billing.ServicePaymentObservation{}, billing.ErrReconciliationRequired
	}
	return p.QueryServicePayment(ctx, o)
}
func (p *WeChat) operationObservation(o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, state, reference, reason string, at time.Time) billing.ServiceOperationObservation {
	v := billing.ServiceOperationObservation{ProfileVersion: p.Profile().Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: o.Payment.TransactionID, ProviderRequestID: op.ProviderRequestID, Kind: op.Reservation.Kind, AmountMinor: op.Reservation.AmountMinor, State: state, ProviderReference: reference, Reason: reason, OccurredAt: money.NormalizeTimestamp(at), VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}
	v.EventID = "service-effect:" + money.ServiceFingerprint(v)
	return v
}
func (p *WeChat) DispatchServiceOperation(ctx context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	var empty billing.ServiceOperationObservation
	if !p.matches(o) || o.Payment == nil || !op.Dispatched || op.Reservation.Validate() != nil || op.Reservation.OrderID != o.Source.OrderID || op.Reservation.AmountMinor <= 0 {
		return empty, billing.ErrConflict
	}
	bm := gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID, "transaction_id": o.Payment.TransactionID, "out_order_no": op.ProviderRequestID}
	switch op.Reservation.Kind {
	case money.ServiceShare:
		if p.now().Before(o.Payment.OccurredAt.Add(30 * time.Second)) {
			return empty, billing.ErrReconciliationRequired
		}
		bm.Set("appid", p.Profile().AppID).Set("finish", false).Set("receivers", []gopay.BodyMap{{"type": "MERCHANT_ID", "receiver_account": p.Profile().PlatformMerchantID, "amount": op.Reservation.AmountMinor, "description": "生态服务平台佣金"}})
		rsp, err := p.client.V3EcommerceProfitShare(ctx, bm)
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			return p.dispatchFailure(o, op, rspSignShare(rsp), rspCodeShare(rsp), err)
		}
	case money.ServiceFinish, money.ServiceRefundRelease:
		description := "客户验收完成生态服务结算"
		if op.Reservation.Kind == money.ServiceRefundRelease {
			description = "已批准生态服务退款解冻"
		}
		bm.Set("description", description)
		rsp, err := p.client.V3EcommerceProfitShareFinish(ctx, bm)
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			if err == nil && rsp != nil {
				return p.dispatchFailure(o, op, rsp.SignInfo, rsp.ErrResponse.Code, err)
			}
			return empty, billing.ErrReconciliationRequired
		}
	case money.ServiceReturn:
		bm = gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID, "out_order_no": op.OriginalShareRequestID, "out_return_no": op.ProviderRequestID, "return_mchid": p.Profile().PlatformMerchantID, "amount": op.Reservation.AmountMinor, "description": "生态服务原平台佣金回退"}
		rsp, err := p.client.V3EcommerceProfitShareReturn(ctx, bm)
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			if err == nil && rsp != nil {
				return p.dispatchFailure(o, op, rsp.SignInfo, rsp.ErrResponse.Code, err)
			}
			return empty, billing.ErrReconciliationRequired
		}
	case money.ServiceRefund:
		bm = gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID, "sp_appid": p.Profile().AppID, "transaction_id": o.Payment.TransactionID, "out_refund_no": op.ProviderRequestID, "reason": "生态服务原路退款", "refund_account": "REFUND_SOURCE_SUB_MERCHANT", "amount": gopay.BodyMap{"refund": op.Reservation.AmountMinor, "total": o.Source.AmountMinor, "currency": "CNY"}}
		if o.Effects[o.ShareOperationID].AmountMinor > 0 || hasConfirmedServiceRelease(o) {
			bm.Set("funds_account", "AVAILABLE")
		}
		rsp, err := p.client.V3EcommerceRefund(ctx, bm)
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			if err == nil && rsp != nil {
				return p.dispatchFailure(o, op, rsp.SignInfo, rsp.ErrResponse.Code, err)
			}
			return empty, billing.ErrReconciliationRequired
		}
	default:
		return empty, billing.ErrInvalid
	}
	return p.QueryServiceOperation(ctx, o, op)
}
func rspSignShare(r *wechat.EcommerceProfitShareRsp) *wechat.SignInfo {
	if r == nil {
		return nil
	}
	return r.SignInfo
}
func rspCodeShare(r *wechat.EcommerceProfitShareRsp) string {
	if r == nil {
		return ""
	}
	return r.ErrResponse.Code
}
func (p *WeChat) dispatchFailure(o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, si *wechat.SignInfo, code string, err error) (billing.ServiceOperationObservation, error) {
	if err == nil && code == "NOT_ENOUGH" && p.verifiedError(si, code) {
		reason := "ORIGINAL_SUBMERCHANT_FUNDS_INSUFFICIENT"
		if op.Reservation.Kind == money.ServiceReturn {
			reason = "ORIGINAL_PLATFORM_RETURN_FUNDS_INSUFFICIENT"
		}
		if op.Reservation.Kind == money.ServiceFinish || op.Reservation.Kind == money.ServiceRefundRelease {
			reason = "ORIGINAL_UNSPLIT_FUNDS_UNAVAILABLE"
		}
		return p.operationObservation(o, op, "WAITING_FUNDS", "", reason, time.Time{}), nil
	}
	if err == nil && (code == "NO_AUTH" || code == "PARAM_ERROR" || code == "RULE_LIMIT") && p.verifiedError(si, code) {
		at, _ := providerResponseTime(si)
		return p.operationObservation(o, op, "FAILED", "wechat-rejected:"+op.ProviderRequestID, "CHANNEL_REJECTED_"+code, at), nil
	}
	return billing.ServiceOperationObservation{}, billing.ErrReconciliationRequired
}
func (p *WeChat) QueryServiceOperation(ctx context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation) (billing.ServiceOperationObservation, error) {
	var empty billing.ServiceOperationObservation
	if !p.matches(o) || o.Payment == nil || op.Reservation.OrderID != o.Source.OrderID {
		return empty, billing.ErrConflict
	}
	switch op.Reservation.Kind {
	case money.ServiceShare, money.ServiceFinish, money.ServiceRefundRelease:
		rsp, err := p.client.V3EcommerceProfitShareQuery(ctx, gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID, "transaction_id": o.Payment.TransactionID, "out_order_no": op.ProviderRequestID})
		if err == nil && rsp != nil && rsp.Code == http.StatusNotFound && rsp.ErrResponse.Code == "RESOURCE_NOT_EXISTS" && p.verifiedError(rsp.SignInfo, "RESOURCE_NOT_EXISTS") {
			return p.originalReplayProof(o, op, rsp.SignInfo)
		}
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			return empty, billing.ErrReconciliationRequired
		}
		at, err := providerResponseTime(rsp.SignInfo)
		if err != nil {
			return empty, err
		}
		return p.shareResult(o, op, *rsp.Response, at)
	case money.ServiceReturn:
		rsp, err := p.client.V3EcommerceProfitShareReturnResult(ctx, gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID, "out_order_no": op.OriginalShareRequestID, "out_return_no": op.ProviderRequestID})
		if err == nil && rsp != nil && rsp.Code == http.StatusNotFound && rsp.ErrResponse.Code == "RESOURCE_NOT_EXISTS" && p.verifiedError(rsp.SignInfo, "RESOURCE_NOT_EXISTS") {
			return p.originalReplayProof(o, op, rsp.SignInfo)
		}
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			return empty, billing.ErrReconciliationRequired
		}
		v := rsp.Response
		if v.SubMchid != o.Source.ProviderMerchantID || v.OutOrderNo != op.OriginalShareRequestID || v.OutReturnNo != op.ProviderRequestID || v.ReturnMchid != p.Profile().PlatformMerchantID || int64(v.Amount) != op.Reservation.AmountMinor || v.OrderId == "" || v.ReturnNo == "" {
			return empty, billing.ErrConflict
		}
		state := "PENDING"
		reason := ""
		var at time.Time
		switch v.Result {
		case "SUCCESS":
			state = "SUCCESS"
			at, _ = time.Parse(time.RFC3339, v.FinishTime)
		case "FAILED":
			state = "FAILED"
			reason = "ORIGINAL_COMMISSION_RETURN_FAILED"
			at, _ = providerResponseTime(rsp.SignInfo)
		case "PROCESSING":
		default:
			return empty, billing.ErrReconciliationRequired
		}
		result := p.operationObservation(o, op, state, "wechat-return:"+v.ReturnNo, reason, at)
		if !result.Matches(o, op) {
			return empty, billing.ErrConflict
		}
		return result, nil
	case money.ServiceRefund:
		rsp, err := p.client.V3EcommerceRefundQueryByNo(ctx, op.ProviderRequestID, gopay.BodyMap{"sub_mchid": o.Source.ProviderMerchantID})
		if err == nil && rsp != nil && rsp.Code == http.StatusNotFound && rsp.ErrResponse.Code == "RESOURCE_NOT_EXISTS" && p.verifiedError(rsp.SignInfo, "RESOURCE_NOT_EXISTS") {
			return p.originalReplayProof(o, op, rsp.SignInfo)
		}
		if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
			return empty, billing.ErrReconciliationRequired
		}
		return p.refundResult(o, op, *rsp.Response)
	}
	return empty, billing.ErrInvalid
}
func (p *WeChat) shareResult(o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, v wechat.EcommerceProfitShareQuery, observed time.Time) (billing.ServiceOperationObservation, error) {
	var empty billing.ServiceOperationObservation
	if o.Payment == nil || v.SubMchid != o.Source.ProviderMerchantID || v.TransactionId != o.Payment.TransactionID || v.OutOrderNo != op.ProviderRequestID || v.OrderId == "" {
		return empty, billing.ErrConflict
	}
	state := "PENDING"
	ref := "wechat-share:" + v.OrderId
	reason := ""
	var at time.Time
	if v.Status != "FINISHED" && v.Status != "PROCESSING" {
		return empty, billing.ErrReconciliationRequired
	}
	switch op.Reservation.Kind {
	case money.ServiceShare:
		if len(v.Receivers) != 1 || v.Receivers[0] == nil {
			return empty, billing.ErrConflict
		}
		r := v.Receivers[0]
		if r.Type != "MERCHANT_ID" || r.ReceiverAccount != p.Profile().PlatformMerchantID || r.ReceiverMchid != "" && r.ReceiverMchid != p.Profile().PlatformMerchantID || int64(r.Amount) != op.Reservation.AmountMinor {
			return empty, billing.ErrConflict
		}
		switch r.Result {
		case "SUCCESS":
			if v.Status != "FINISHED" || r.DetailId == "" {
				return empty, billing.ErrConflict
			}
			state = "SUCCESS"
			ref = "wechat-share-detail:" + r.DetailId
			at, _ = time.Parse(time.RFC3339, r.FinishTime)
		case "CLOSED":
			state = "FAILED"
			reason = "PLATFORM_COMMISSION_RECEIVER_CLOSED"
			at = observed
		case "PENDING":
		default:
			return empty, billing.ErrReconciliationRequired
		}
	case money.ServiceFinish, money.ServiceRefundRelease:
		if len(v.Receivers) != 0 {
			return empty, billing.ErrConflict
		}
		if v.Status == "FINISHED" {
			if int64(v.FinishAmount) != op.Reservation.AmountMinor {
				return empty, billing.ErrConflict
			}
			state = "SUCCESS"
			ref = "wechat-finish:" + v.OrderId
			at = observed
		}
	default:
		return empty, billing.ErrInvalid
	}
	result := p.operationObservation(o, op, state, ref, reason, at)
	if !result.Matches(o, op) {
		return empty, billing.ErrConflict
	}
	return result, nil
}
func (p *WeChat) refundResult(o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, v wechat.EcommerceRefundQuery) (billing.ServiceOperationObservation, error) {
	var empty billing.ServiceOperationObservation
	if op.Reservation.Kind != money.ServiceRefund || o.Payment == nil || v.TransactionId != o.Payment.TransactionID || v.OutTradeNo != o.TradeNo || v.OutRefundNo != op.ProviderRequestID || v.RefundId == "" || v.RefundAccount != "REFUND_SOURCE_SUB_MERCHANT" || v.Amount == nil || v.Amount.Currency != "CNY" || int64(v.Amount.Refund) != op.Reservation.AmountMinor || v.Amount.DiscountRefund != 0 || v.Amount.PayerRefund != v.Amount.Refund {
		return empty, billing.ErrConflict
	}
	state := "PENDING"
	reason := ""
	var at time.Time
	switch v.Status {
	case "SUCCESS":
		state = "SUCCESS"
		at, _ = time.Parse(time.RFC3339, v.SuccessTime)
	case "CLOSED":
		state = "FAILED"
		reason = "ORIGINAL_CUSTOMER_REFUND_CLOSED"
		at, _ = time.Parse(time.RFC3339, v.CreateTime)
	case "PROCESSING":
	case "ABNORMAL":
		state = "WAITING_FUNDS"
		reason = "ORIGINAL_CUSTOMER_REFUND_ABNORMAL"
	default:
		return empty, billing.ErrReconciliationRequired
	}
	result := p.operationObservation(o, op, state, "wechat-refund:"+v.RefundId, reason, at)
	if !result.Matches(o, op) {
		return empty, billing.ErrConflict
	}
	return result, nil
}
func providerResponseTime(s *wechat.SignInfo) (time.Time, error) {
	if s == nil {
		return time.Time{}, billing.ErrReconciliationRequired
	}
	v, err := strconv.ParseInt(s.HeaderTimestamp, 10, 64)
	if err != nil || v <= 0 {
		return time.Time{}, billing.ErrReconciliationRequired
	}
	return time.Unix(v, 0).UTC(), nil
}
func (p *WeChat) verifiedError(s *wechat.SignInfo, expected string) bool {
	return paymentsecurity.VerifiedWeChatError(s, p.config.PublicKeyID, p.publicKey, p.now(), expected)
}
func hasConfirmedServiceRelease(o billing.ServicePurchaseOrder) bool {
	for _, receipt := range o.Effects {
		if (receipt.Kind == money.ServiceFinish || receipt.Kind == money.ServiceRefundRelease) && receipt.AmountMinor > 0 {
			return true
		}
	}
	return false
}
func (p *WeChat) originalReplayProof(o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, si *wechat.SignInfo) (billing.ServiceOperationObservation, error) {
	at, err := providerResponseTime(si)
	if err != nil {
		return billing.ServiceOperationObservation{}, err
	}
	return p.operationObservation(o, op, "REPLAY_ALLOWED", "wechat-absence:"+money.ServiceFingerprint(si.SignBody), "VERIFIED_ORIGINAL_REQUEST_ABSENT", at), nil
}
