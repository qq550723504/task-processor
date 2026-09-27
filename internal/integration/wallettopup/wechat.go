package wallettopup

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-pay/crypto/xpem"
	"github.com/go-pay/gopay"
	wechat "github.com/go-pay/gopay/wechat/v3"

	"task-processor/internal/commercial/billing"
)

type WeChatConfig struct {
	Merchant                                                              billing.TopUpMerchant
	NewPayments                                                           bool
	PrivateKey, SerialNumber, APIv3Key, PublicKeyID, PublicKey, NotifyURL string
}
type WeChat struct {
	client    *wechat.ClientV3
	config    WeChatConfig
	publicKey *rsa.PublicKey
	now       func() time.Time
}

func NewWeChat(cfg WeChatConfig) (*WeChat, error) {
	if cfg.Merchant.Validate() != nil || cfg.Merchant.Provider != billing.PaymentWeChat || !validHTTPS(cfg.NotifyURL) || !strings.HasPrefix(cfg.PublicKeyID, "PUB_KEY_ID_") || cfg.SerialNumber == "" || len(cfg.APIv3Key) != 32 {
		return nil, billing.ErrInvalid
	}
	key, err := xpem.DecodePublicKey([]byte(cfg.PublicKey))
	if err != nil || key == nil || key.N.BitLen() < 2048 {
		return nil, billing.ErrInvalid
	}
	client, err := wechat.NewClientV3(cfg.Merchant.MerchantID, cfg.SerialNumber, cfg.APIv3Key, cfg.PrivateKey)
	if err != nil {
		return nil, billing.ErrInvalid
	}
	if err = client.AutoVerifySignByPublicKey([]byte(cfg.PublicKey), cfg.PublicKeyID); err != nil {
		return nil, billing.ErrInvalid
	}
	client.SetHttpClient(providerHTTP())
	client.SetBodySize(1)
	client.SetLogger(providerSilentLogger{})
	return &WeChat{client: client, config: cfg, publicKey: key, now: time.Now}, nil
}
func (p *WeChat) Merchant() billing.TopUpMerchant { return p.config.Merchant }
func (p *WeChat) Available() bool                 { return p.config.NewPayments }
func (p *WeChat) CreateOrReadCheckout(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.CheckoutAction, error) {
	remaining := a.ExpiresAt.Sub(p.now())
	if !p.Available() || a.Merchant != p.Merchant() || a.Validate() != nil || remaining < 75*time.Second || remaining > 2*time.Hour {
		return billing.CheckoutAction{}, billing.ErrPaymentMethodUnavailable
	}
	// A short expiry is extended by the provider to one minute. Reject that
	// boundary instead of silently changing the original attempt's deadline.
	rsp, err := p.client.V3TransactionNative(ctx, gopay.BodyMap{"appid": p.Merchant().AppID, "mchid": p.Merchant().MerchantID, "description": "企业钱包充值", "out_trade_no": a.MerchantOrderID, "time_expire": a.ExpiresAt.Format(time.RFC3339), "notify_url": p.config.NotifyURL, "amount": gopay.BodyMap{"total": a.AmountMinor, "currency": "CNY"}})
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
		return billing.CheckoutAction{}, billing.ErrReconciliationRequired
	}
	u, err := url.Parse(rsp.Response.CodeUrl)
	if err != nil || u.Scheme != "weixin" || u.Host != "wxpay" || u.Path != "/bizpayurl" || u.User != nil || u.Fragment != "" || u.Query().Get("pr") == "" {
		return billing.CheckoutAction{}, billing.ErrReconciliationRequired
	}
	return action(a, "QR_CODE", rsp.Response.CodeUrl), nil
}
func (p *WeChat) payment(v wechat.QueryOrder) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	paid := v.TradeState == "SUCCESS" || v.TradeState == "REFUND"
	if v.Appid != p.Merchant().AppID || v.Mchid != p.Merchant().MerchantID || (v.TradeType != "" && v.TradeType != "NATIVE") || (paid && (v.TradeType != "NATIVE" || v.Amount == nil)) || (v.Amount != nil && v.Amount.Currency != "CNY") {
		return empty, billing.ErrConflict
	}
	var amount int64
	if v.Amount != nil {
		amount = int64(v.Amount.Total)
	}
	paidAt, _ := time.Parse(time.RFC3339, v.SuccessTime)
	o := billing.ProviderObservation{Merchant: p.Merchant(), MerchantOrderID: v.OutTradeNo, Kind: "PAYMENT", State: "UNKNOWN", TradeID: v.TransactionId, Currency: "CNY", AmountMinor: amount, OccurredAt: paidAt, VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}
	switch v.TradeState {
	case "SUCCESS", "REFUND":
		o.State = "PAID"
	case "NOTPAY", "USERPAYING":
		o.State = "UNPAID"
	case "CLOSED":
		o.State = "CLOSED"
	}
	o = observationID(o)
	if o.Validate() != nil {
		return empty, billing.ErrReconciliationRequired
	}
	return o, nil
}
func (p *WeChat) QueryPayment(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	if a.Merchant != p.Merchant() {
		return empty, billing.ErrConflict
	}
	rsp, err := p.client.V3TransactionQueryOrder(ctx, wechat.OutTradeNo, a.MerchantOrderID)
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
		return empty, billing.ErrReconciliationRequired
	}
	o, err := p.payment(*rsp.Response)
	if err != nil {
		return empty, err
	}
	if !o.Matches(a) {
		return empty, billing.ErrConflict
	}
	return o, nil
}
func (p *WeChat) ClosePayment(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	if a.Merchant != p.Merchant() {
		return billing.ProviderObservation{}, billing.ErrConflict
	}
	rsp, err := p.client.V3TransactionCloseOrder(ctx, a.MerchantOrderID)
	if err != nil || rsp == nil || rsp.Code != 0 {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	return p.QueryPayment(ctx, a)
}
func (p *WeChat) Refund(ctx context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	if r.Merchant != p.Merchant() || !r.Dispatched || r.TotalMinor <= 0 {
		return billing.ProviderObservation{}, billing.ErrConflict
	}
	rsp, err := p.client.V3Refund(ctx, gopay.BodyMap{"transaction_id": r.TradeID, "out_refund_no": r.ProviderRequestID, "reason": "企业钱包充值原路退款", "notify_url": p.config.NotifyURL, "amount": gopay.BodyMap{"refund": r.AmountMinor, "total": r.TotalMinor, "currency": "CNY"}})
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	// The query, including its signed original trade binding, is the acceptance fact.
	return p.QueryRefund(ctx, r)
}
func (p *WeChat) refund(order, trade, request, native, state, paidAt string, total, amount int64) (billing.ProviderObservation, error) {
	occurred, _ := time.Parse(time.RFC3339, paidAt)
	o := billing.ProviderObservation{Merchant: p.Merchant(), MerchantOrderID: order, Kind: "REFUND", State: "REFUND_PENDING", TradeID: trade, RefundRequestID: request, NativeRefundID: native, Currency: "CNY", AmountMinor: amount, TotalMinor: total, OccurredAt: occurred, VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}
	switch state {
	case "SUCCESS":
		o.State = "REFUNDED"
	case "CLOSED":
		o.State = "REFUND_CLOSED"
	case "PROCESSING", "ABNORMAL":
	default:
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	o = observationID(o)
	if o.Validate() != nil || native == "" {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	return o, nil
}
func (p *WeChat) QueryRefund(ctx context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	if r.Merchant != p.Merchant() {
		return empty, billing.ErrConflict
	}
	rsp, err := p.client.V3RefundQuery(ctx, r.ProviderRequestID, nil)
	if err != nil || rsp == nil || rsp.Code != 0 || rsp.Response == nil || rsp.Response.Amount == nil || rsp.Response.Amount.Currency != "CNY" {
		return empty, billing.ErrReconciliationRequired
	}
	v := rsp.Response
	if v.OutTradeNo != r.MerchantOrderID || v.TransactionId != r.TradeID || v.OutRefundNo != r.ProviderRequestID {
		return empty, billing.ErrConflict
	}
	return p.refund(v.OutTradeNo, v.TransactionId, v.OutRefundNo, v.RefundId, v.Status, v.SuccessTime, int64(v.Amount.Total), int64(v.Amount.Refund))
}
func (p *WeChat) VerifyNotification(req *http.Request) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	raw, err := callbackBody(req, "application/json")
	if err != nil {
		return empty, err
	}
	for _, name := range []string{"Wechatpay-Timestamp", "Wechatpay-Nonce", "Wechatpay-Signature", "Wechatpay-Serial"} {
		if len(req.Header.Values(name)) != 1 || req.Header.Get(name) == "" {
			return empty, billing.ErrInvalid
		}
	}
	if req.Header.Get("Wechatpay-Serial") != p.config.PublicKeyID {
		return empty, billing.ErrInvalid
	}
	ts, err := strconv.ParseInt(req.Header.Get("Wechatpay-Timestamp"), 10, 64)
	if err != nil || time.Unix(ts, 0).Before(p.now().Add(-5*time.Minute)) || time.Unix(ts, 0).After(p.now().Add(5*time.Minute)) {
		return empty, billing.ErrInvalid
	}
	if wechat.V3VerifySignByPK(req.Header.Get("Wechatpay-Timestamp"), req.Header.Get("Wechatpay-Nonce"), string(raw), req.Header.Get("Wechatpay-Signature"), p.publicKey) != nil {
		return empty, billing.ErrInvalid
	}
	var envelope struct {
		ID           string           `json:"id"`
		EventType    string           `json:"event_type"`
		ResourceType string           `json:"resource_type"`
		Resource     *wechat.Resource `json:"resource"`
	}
	if strictJSON(raw, &envelope) != nil || envelope.ResourceType != "encrypt-resource" || envelope.Resource == nil || envelope.Resource.Algorithm != "AEAD_AES_256_GCM" || len(envelope.Resource.Nonce) != 12 {
		return empty, billing.ErrInvalid
	}
	plain, err := wechat.V3DecryptNotifyCipherTextToBytes(envelope.Resource.Ciphertext, envelope.Resource.Nonce, envelope.Resource.AssociatedData, p.config.APIv3Key)
	if err != nil {
		return empty, billing.ErrInvalid
	}
	var o billing.ProviderObservation
	switch envelope.EventType {
	case "TRANSACTION.SUCCESS":
		var v wechat.QueryOrder
		if strictJSON(plain, &v) != nil || v.TradeState != "SUCCESS" {
			return empty, billing.ErrInvalid
		}
		o, err = p.payment(v)
	case "REFUND.SUCCESS", "REFUND.CLOSED", "REFUND.ABNORMAL":
		var v wechat.V3DecryptRefundResult
		if strictJSON(plain, &v) != nil || v.Mchid != p.Merchant().MerchantID || v.Amount == nil || envelope.EventType != "REFUND."+v.RefundStatus {
			return empty, billing.ErrInvalid
		}
		// Native CNY refund notifications omit appid and currency. This callback
		// profile is bound to the original merchant; the order worker checks total/trade.
		o, err = p.refund(v.OutTradeNo, v.TransactionId, v.OutRefundNo, v.RefundId, v.RefundStatus, v.SuccessTime, int64(v.Amount.Total), int64(v.Amount.Refund))
	default:
		return empty, billing.ErrInvalid
	}
	if err != nil {
		return empty, err
	}
	o.EventID = envelope.ID
	if o.Validate() != nil {
		return empty, billing.ErrInvalid
	}
	return o, nil
}
