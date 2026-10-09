package servicepayments

import (
	"bytes"
	"context"
	"crypto/sha1" // Official signed bill manifest uses SHA1 for file integrity.
	"encoding/csv"
	"encoding/hex"
	"errors"
	"github.com/go-pay/gopay"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	b "task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	m "task-processor/internal/ledger/money"
	"time"
	"unicode/utf8"
)

var channelBillZone = time.FixedZone("WeChat billing", 8*60*60)

func (p *WeChat) QueryServiceFees(ctx context.Context, o b.ServicePurchaseOrder, date string) ([]m.ServiceChannelFee, error) {
	if !p.matches(o) || o.Payment == nil || !p.config.ProductQualified || !p.config.PlatformPaysFees {
		return nil, b.ErrFeatureUnavailable
	}
	day, err := time.ParseInLocation("2006-01-02", date, channelBillZone)
	now := p.now().In(channelBillZone)
	if err != nil || day.Format("2006-01-02") != date || date >= now.Format("2006-01-02") || day.Before(now.AddDate(0, -3, 0).Truncate(24*time.Hour)) || date < o.Payment.OccurredAt.In(channelBillZone).Format("2006-01-02") {
		return nil, b.ErrInvalid
	}
	response, err := p.client.V3BillFundFlowBill(ctx, gopay.BodyMap{"bill_date": date, "account_type": "FEES"})
	if err != nil || response == nil || response.Code != 0 || response.Response == nil || !paymentsecurity.VerifiedWeChatResponse(response.SignInfo, p.config.PublicKeyID, p.publicKey, p.now()) {
		return nil, b.ErrReconciliationRequired
	}
	var manifest struct {
		HashType string `json:"hash_type"`
		Hash     string `json:"hash_value"`
		URL      string `json:"download_url"`
	}
	if paymentsecurity.StrictJSON([]byte(response.SignInfo.SignBody), &manifest) != nil || manifest.HashType != "SHA1" || len(manifest.Hash) != 40 {
		return nil, b.ErrReconciliationRequired
	}
	target, err := url.Parse(manifest.URL)
	if err != nil || len(manifest.URL) > 2048 || target.Scheme != "https" || target.Host != "api.mch.weixin.qq.com" || target.Path != "/v3/bill/downloadurl" || target.User != nil || target.Fragment != "" || target.Query().Get("token") == "" || len(target.Query()) != 1 || len(target.Query()["token"]) != 1 {
		return nil, b.ErrReconciliationRequired
	}
	raw, err := p.client.V3BillDownLoadBill(ctx, manifest.URL)
	if err != nil || len(raw) > 1024*1024 {
		return nil, b.ErrReconciliationRequired
	}
	digest := sha1.Sum(raw)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), manifest.Hash) {
		return nil, b.ErrReconciliationRequired
	}
	proof := "wechat-fees-bill:" + m.ServiceFingerprint([]string{o.Profile.Version, o.Profile.PlatformMerchantID, date, manifest.Hash})
	facts, err := p.parseFeeBill(o, date, raw, proof)
	if err != nil {
		return nil, err
	}
	if len(facts) == 0 {
		return nil, b.ErrNotFound
	}
	return facts, nil
}

func (p *WeChat) parseFeeBill(o b.ServicePurchaseOrder, date string, raw []byte, proof string) ([]m.ServiceChannelFee, error) {
	if !p.matches(o) || o.Payment == nil || !utf8.Valid(raw) || len(raw) > 1024*1024 {
		return nil, b.ErrInvalid
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, b.ErrInvalid
	}
	columns := map[string]int{}
	for i, name := range header {
		name = strings.TrimPrefix(name, "`")
		if _, exists := columns[name]; exists {
			return nil, b.ErrInvalid
		}
		columns[name] = i
	}
	required := []string{"记账时间", "微信支付业务单号", "资金流水单号", "业务类型", "收支类型", "收支金额(元)"}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return nil, b.ErrInvalid
		}
	}
	original := map[string]bool{o.Payment.TransactionID: true}
	for _, effect := range o.Effects {
		if effect.Kind == m.ServiceRefund && strings.HasPrefix(effect.ProviderReference, "wechat-refund:") {
			original[strings.TrimPrefix(effect.ProviderReference, "wechat-refund:")] = true
		}
	}
	var facts []m.ServiceChannelFee
	seen := map[string]bool{}
	summary := false
	for n := 0; n < 100000; n++ {
		row, e := reader.Read()
		if errors.Is(e, io.EOF) {
			if !summary {
				return nil, b.ErrInvalid
			}
			return facts, nil
		}
		if e != nil {
			return nil, b.ErrInvalid
		}
		if len(row) > 0 && strings.TrimPrefix(row[0], "`") == "资金流水总笔数" {
			summary = true
			continue
		}
		if summary {
			continue
		}
		if len(row) != len(header) {
			return nil, b.ErrInvalid
		}
		get := func(name string) string { return strings.TrimPrefix(row[columns[name]], "`") }
		business := get("微信支付业务单号")
		if !original[business] {
			continue
		}
		kind := get("业务类型")
		if kind != "扣除交易手续费" && kind != "退还交易手续费" {
			continue
		}
		returned := kind == "退还交易手续费"
		if get("收支类型") != map[bool]string{false: "支出", true: "收入"}[returned] {
			return nil, b.ErrConflict
		}
		at, e := time.ParseInLocation("2006-01-02 15:04:05", get("记账时间"), channelBillZone)
		if e != nil || at.Format("2006-01-02") != date {
			return nil, b.ErrConflict
		}
		amount, e := feeMinor(get("收支金额(元)"))
		if e != nil {
			return nil, e
		}
		flow := get("资金流水单号")
		if seen[flow] {
			return nil, b.ErrConflict
		}
		seen[flow] = true
		fact := m.ServiceChannelFee{Payment: o.MoneyInput(), FlowID: flow, BusinessID: business, ProofID: proof, AmountMinor: amount, Returned: returned, OccurredAt: at.UTC()}
		if fact.Validate() != nil {
			return nil, b.ErrConflict
		}
		facts = append(facts, fact)
	}
	return nil, b.ErrInvalid
}
func feeMinor(raw string) (int64, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(parts[1]) != 2 || len(parts[0]) < 1 || len(parts[0]) > 17 {
		return 0, b.ErrInvalid
	}
	for _, r := range parts[0] + parts[1] {
		if r < '0' || r > '9' {
			return 0, b.ErrInvalid
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > math.MaxInt64/100 {
		return 0, b.ErrInvalid
	}
	fraction, _ := strconv.ParseInt(parts[1], 10, 64)
	if whole == math.MaxInt64/100 && fraction > math.MaxInt64%100 {
		return 0, b.ErrInvalid
	}
	return whole*100 + fraction, nil
}
