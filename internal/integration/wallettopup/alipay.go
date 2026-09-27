package wallettopup

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-pay/crypto/xpem"
	"github.com/go-pay/crypto/xrsa"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/pkg/xhttp"

	"task-processor/internal/commercial/billing"
)

type AlipayConfig struct {
	Merchant                                    billing.TopUpMerchant
	NewPayments                                 bool
	PrivateKey, PublicKey, NotifyURL, ReturnURL string
}
type Alipay struct {
	client *alipay.Client
	config AlipayConfig
	now    func() time.Time
}

func NewAlipay(cfg AlipayConfig) (*Alipay, error) {
	cfg.PublicKey = aliKey(cfg.PublicKey)
	cfg.PrivateKey = aliKey(cfg.PrivateKey)
	if cfg.Merchant.Validate() != nil || cfg.Merchant.Provider != billing.PaymentAlipay || !validHTTPS(cfg.NotifyURL) || !validHTTPS(cfg.ReturnURL) || cfg.PublicKey == "" {
		return nil, billing.ErrInvalid
	}
	key, err := xpem.DecodePublicKey([]byte(xrsa.FormatAlipayPublicKey(cfg.PublicKey)))
	if err != nil || key == nil || key.N.BitLen() < 2048 {
		return nil, billing.ErrInvalid
	}
	client, err := alipay.NewClient(cfg.Merchant.AppID, cfg.PrivateKey, cfg.Merchant.Environment == "PRODUCTION")
	if err != nil {
		return nil, billing.ErrInvalid
	}
	client.SetNotifyUrl(cfg.NotifyURL).SetReturnUrl(cfg.ReturnURL).SetSignType(alipay.RSA2)
	client.SetHttpClient(providerHTTP())
	client.SetBodySize(1)
	client.SetLogger(providerSilentLogger{})
	return &Alipay{client: client, config: cfg, now: time.Now}, nil
}
func aliKey(raw string) string {
	if block, rest := pem.Decode([]byte(raw)); block != nil && strings.TrimSpace(string(rest)) == "" {
		return base64.StdEncoding.EncodeToString(block.Bytes)
	}
	return strings.Join(strings.Fields(raw), "")
}
func providerHTTP() *xhttp.Client {
	c := xhttp.NewClient().SetTimeout(10 * time.Second)
	c.HttpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return billing.ErrInvalid }
	return c
}
func (p *Alipay) Merchant() billing.TopUpMerchant { return p.config.Merchant }
func (p *Alipay) Available() bool                 { return p.config.NewPayments }
func (p *Alipay) CreateOrReadCheckout(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.CheckoutAction, error) {
	if !p.Available() || a.Merchant != p.Merchant() || a.Validate() != nil || !p.now().Before(a.ExpiresAt) {
		return billing.CheckoutAction{}, billing.ErrPaymentMethodUnavailable
	}
	bm := gopay.BodyMap{"out_trade_no": a.MerchantOrderID, "seller_id": p.Merchant().MerchantID, "total_amount": formatYuan(a.AmountMinor), "subject": "企业钱包充值", "time_expire": a.ExpiresAt.In(chinaZone).Format("2006-01-02 15:04:05")}
	raw, err := p.client.TradePagePay(ctx, bm)
	if err != nil {
		return billing.CheckoutAction{}, billing.ErrReconciliationRequired
	}
	u, err := url.Parse(raw)
	host := "openapi.alipay.com"
	if p.Merchant().Environment == "SANDBOX" {
		host = "openapi-sandbox.dl.alipaydev.com"
	}
	if err != nil || u.Scheme != "https" || u.Host != host || u.Path != "/gateway.do" || u.User != nil || u.Fragment != "" {
		return billing.CheckoutAction{}, billing.ErrReconciliationRequired
	}
	return action(a, "REDIRECT", raw), nil
}

var chinaZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func aliTime(raw string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", raw, chinaZone)
	return t.UTC()
}
func (p *Alipay) verified(data, sign string) bool {
	var obj any
	if strictJSON([]byte(data), &obj) != nil {
		return false
	}
	ok, err := alipay.VerifySyncSign(p.config.PublicKey, data, sign)
	return err == nil && ok
}
func (p *Alipay) QueryPayment(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	if a.Merchant != p.Merchant() {
		return empty, billing.ErrConflict
	}
	rsp, err := p.client.TradeQuery(ctx, gopay.BodyMap{"out_trade_no": a.MerchantOrderID})
	if err != nil || rsp == nil || rsp.Response == nil || !p.verified(rsp.SignData, rsp.Sign) || rsp.Response.Code != "10000" {
		return empty, billing.ErrReconciliationRequired
	}
	v := rsp.Response
	if v.OutTradeNo != a.MerchantOrderID || (v.TransCurrency != "" && v.TransCurrency != "CNY") || (v.PayCurrency != "" && v.PayCurrency != "CNY") {
		return empty, billing.ErrConflict
	}
	amount, err := parseYuan(v.TotalAmount)
	if err != nil || amount != a.AmountMinor {
		return empty, billing.ErrConflict
	}
	o := billing.ProviderObservation{Merchant: p.Merchant(), MerchantOrderID: v.OutTradeNo, Kind: "PAYMENT", State: "UNKNOWN", TradeID: v.TradeNo, Currency: "CNY", AmountMinor: amount, OccurredAt: aliTime(v.SendPayDate), VerificationVersion: "alipay-rsa2-v1"}
	switch v.TradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		o.State = "PAID"
	case "WAIT_BUYER_PAY":
		o.State = "UNPAID"
	case "TRADE_CLOSED":
		if !o.OccurredAt.IsZero() {
			o.State = "PAID"
		} else {
			o.State = "CLOSED"
		}
	}
	o = observationID(o)
	if o.Validate() != nil {
		return empty, billing.ErrReconciliationRequired
	}
	return o, nil
}
func (p *Alipay) ClosePayment(ctx context.Context, a billing.TopUpPaymentAttempt) (billing.ProviderObservation, error) {
	if a.Merchant != p.Merchant() {
		return billing.ProviderObservation{}, billing.ErrConflict
	}
	rsp, err := p.client.TradeClose(ctx, gopay.BodyMap{"out_trade_no": a.MerchantOrderID})
	if err != nil || rsp == nil || rsp.Response == nil || !p.verified(rsp.SignData, rsp.Sign) || rsp.Response.Code != "10000" || rsp.Response.OutTradeNo != a.MerchantOrderID {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	// Closing alone never proves no earlier payment or refund. Query the original trade.
	return p.QueryPayment(ctx, a)
}
func (p *Alipay) Refund(ctx context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	if r.Merchant != p.Merchant() || !r.Dispatched {
		return billing.ProviderObservation{}, billing.ErrConflict
	}
	rsp, err := p.client.TradeRefund(ctx, gopay.BodyMap{"out_trade_no": r.MerchantOrderID, "trade_no": r.TradeID, "out_request_no": r.ProviderRequestID, "refund_amount": formatYuan(r.AmountMinor), "refund_reason": "企业钱包充值原路退款"})
	if err != nil || rsp == nil || rsp.Response == nil || !p.verified(rsp.SignData, rsp.Sign) || rsp.Response.Code != "10000" || rsp.Response.OutTradeNo != r.MerchantOrderID || rsp.Response.TradeNo != r.TradeID {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	return p.QueryRefund(ctx, r)
}
func (p *Alipay) QueryRefund(ctx context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	if r.Merchant != p.Merchant() {
		return empty, billing.ErrConflict
	}
	rsp, err := p.client.TradeFastPayRefundQuery(ctx, gopay.BodyMap{"out_trade_no": r.MerchantOrderID, "trade_no": r.TradeID, "out_request_no": r.ProviderRequestID, "query_options": []string{"gmt_refund_pay"}})
	if err != nil || rsp == nil || rsp.Response == nil || !p.verified(rsp.SignData, rsp.Sign) || rsp.Response.Code != "10000" {
		return empty, billing.ErrReconciliationRequired
	}
	v := rsp.Response
	if v.OutTradeNo != r.MerchantOrderID || v.TradeNo != r.TradeID || v.OutRequestNo != r.ProviderRequestID {
		return empty, billing.ErrConflict
	}
	amount, e1 := parseYuan(v.RefundAmount)
	total, e2 := parseYuan(v.TotalAmount)
	if e1 != nil || e2 != nil {
		return empty, billing.ErrConflict
	}
	o := billing.ProviderObservation{Merchant: p.Merchant(), MerchantOrderID: v.OutTradeNo, Kind: "REFUND", State: "REFUND_PENDING", TradeID: v.TradeNo, RefundRequestID: v.OutRequestNo, Currency: "CNY", AmountMinor: amount, TotalMinor: total, OccurredAt: aliTime(v.GmtRefundPay), VerificationVersion: "alipay-rsa2-v1"}
	if v.RefundStatus == "REFUND_SUCCESS" {
		o.State = "REFUNDED"
	}
	o = observationID(o)
	if o.Validate() != nil {
		return empty, billing.ErrReconciliationRequired
	}
	return o, nil
}

// VerifyNotification only verifies and normalizes. The caller must durably store
// this observation before writing "success"; no wallet or provider call runs here.
func (p *Alipay) VerifyNotification(req *http.Request) (billing.ProviderObservation, error) {
	var empty billing.ProviderObservation
	raw, err := callbackBody(req, "application/x-www-form-urlencoded")
	if err != nil {
		return empty, err
	}
	values, err := url.ParseQuery(string(raw))
	if err != nil || len(values) > 100 {
		return empty, billing.ErrInvalid
	}
	for _, v := range values {
		if len(v) != 1 {
			return empty, billing.ErrInvalid
		}
	}
	if values.Get("sign_type") != "RSA2" || values.Get("app_id") != p.Merchant().AppID || values.Get("seller_id") != p.Merchant().MerchantID {
		return empty, billing.ErrInvalid
	}
	bm, err := alipay.ParseNotifyByURLValues(values)
	if err != nil {
		return empty, billing.ErrInvalid
	}
	verified, err := alipay.VerifySign(p.config.PublicKey, bm)
	if err != nil || !verified {
		return empty, billing.ErrInvalid
	}
	amount, err := parseYuan(values.Get("total_amount"))
	if err != nil {
		return empty, err
	}
	o := billing.ProviderObservation{Merchant: p.Merchant(), MerchantOrderID: values.Get("out_trade_no"), EventID: values.Get("notify_id"), Kind: "PAYMENT", State: "UNKNOWN", TradeID: values.Get("trade_no"), Currency: "CNY", AmountMinor: amount, OccurredAt: aliTime(values.Get("gmt_payment")), VerificationVersion: "alipay-rsa2-v1"}
	switch values.Get("trade_status") {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		o.State = "PAID"
	case "WAIT_BUYER_PAY":
		o.State = "UNPAID"
	case "TRADE_CLOSED":
		o.State = "CLOSED"
	default:
		return empty, billing.ErrInvalid
	}
	if values.Get("refund_fee") != "" || values.Get("gmt_refund") != "" {
		// refund_fee can be cumulative. It is only a lookup hint, never an accepted reversal.
		refund, err := parseYuan(values.Get("refund_fee"))
		if err != nil {
			return empty, err
		}
		o.Kind = "REFUND"
		o.State = "REFUND_PENDING"
		o.TotalMinor = amount
		o.AmountMinor = refund
		o.RefundRequestID = values.Get("out_biz_no")
		o.OccurredAt = time.Time{}
	}
	if o.Validate() != nil {
		return empty, billing.ErrInvalid
	}
	return o, nil
}
