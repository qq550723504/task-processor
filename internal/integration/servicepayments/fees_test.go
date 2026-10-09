package servicepayments

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strconv"
	"strings"
	b "task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestOriginalPlatformFeeBillUsesActualFlowAndIntegerYuan(t *testing.T) {
	p, o, _ := wechatServiceFixture()
	o.Source.RequestID = "original-request"
	o.Source.BuyerOrganizationID = "buyer"
	o.Source.ProviderOrganizationID = "provider"
	o.Source.PolicyVersion = "policy"
	o.Payment.OccurredAt = time.Now().UTC()
	bill := "记账时间,微信支付业务单号,资金流水单号,业务名称,业务类型,收支类型,收支金额(元),账户结余(元),资金变更提交申请人,备注,业务凭证号\n" +
		"`2026-10-08 10:00:00,`transaction,`flow-original,`交易,`扣除交易手续费,`支出,`0.03,`100.00,`actor,`memo,`voucher\n" +
		"`2026-10-08 10:01:00,`foreign,`flow-foreign,`交易,`扣除交易手续费,`支出,`9.99,`100.00,`actor,`memo,`voucher\n" +
		"资金流水总笔数,收入笔数,收入金额,支出笔数,支出金额\n`2,`0,`0,`2,`10.02\n"
	facts, err := p.parseFeeBill(o, "2026-10-08", []byte(bill), "verified-bill-hash")
	if err != nil || len(facts) != 1 || facts[0].AmountMinor != 3 || facts[0].FlowID != "flow-original" || facts[0].Returned {
		t.Fatalf("original actual fee lost: %+v %v", facts, err)
	}
	if _, err = p.parseFeeBill(o, "2026-10-09", []byte(bill), "proof"); err == nil {
		t.Fatal("wrong bill date accepted")
	}
	for _, value := range []string{"0.001", "1e2", "-0.03", "9223372036854775807.01"} {
		if _, err = feeMinor(value); err == nil {
			t.Fatalf("ambiguous minor amount accepted %s", value)
		}
	}
}

type feeBillTransport func(*http.Request) (*http.Response, error)

func (f feeBillTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQueryServiceFeesRequiresSignedBoundedOriginalBill(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	fixture, original, _ := wechatServiceFixture()
	cfg := fixture.config
	cfg.PrivateKey, cfg.PublicKey = string(private), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}))
	cfg.SerialNumber, cfg.APIv3Key, cfg.NotifyURL = "fixture-serial", strings.Repeat("a", 32), "https://platform.example/notify"
	cfg.ProductQualified, cfg.PlatformPaysFees = true, true
	now := time.Now().UTC()
	date := now.Add(-24 * time.Hour).In(channelBillZone).Format("2006-01-02")
	original.Source.RequestID, original.Source.BuyerOrganizationID, original.Source.ProviderOrganizationID, original.Source.PolicyVersion = "request", "buyer", "provider", "policy"
	original.Payment.OccurredAt = now.Add(-48 * time.Hour)
	original.Effects = map[string]m.ServiceReceipt{"refund": {Kind: m.ServiceRefund, ProviderReference: "wechat-refund:original-refund"}}
	bill := "记账时间,微信支付业务单号,资金流水单号,业务类型,收支类型,收支金额(元)\n" +
		"`" + date + " 10:00:00,`transaction,`fee-flow,`扣除交易手续费,`支出,`0.03\n" +
		"`" + date + " 10:01:00,`original-refund,`return-flow,`退还交易手续费,`收入,`0.01\n" +
		"`" + date + " 10:02:00,`foreign-refund,`foreign-flow,`退还交易手续费,`收入,`9.99\n" +
		"资金流水总笔数,收入笔数,收入金额,支出笔数,支出金额\n`3,`2,`10.00,`1,`0.03\n"
	for _, scenario := range []string{"verified", "unconfirmed_refund", "invalid_signature", "wrong_hash", "external_url", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			p, e := NewWeChat(cfg)
			if e != nil {
				t.Fatal(e)
			}
			p.now = func() time.Time { return now }
			order := original
			if scenario == "unconfirmed_refund" {
				order.Effects = nil
			}
			file := bill
			if scenario == "oversized" {
				file += strings.Repeat("x", 2*1024*1024)
			}
			hash := sha1.Sum([]byte(file))
			manifest := map[string]string{"hash_type": "SHA1", "hash_value": hex.EncodeToString(hash[:]), "download_url": "https://api.mch.weixin.qq.com/v3/bill/downloadurl?token=original-token"}
			if scenario == "wrong_hash" {
				manifest["hash_value"] = strings.Repeat("0", 40)
			}
			if scenario == "external_url" {
				manifest["download_url"] = "https://foreign.example/v3/bill/downloadurl?token=original-token"
			}
			body, _ := json.Marshal(manifest)
			queries, downloads := 0, 0
			client := paymentsecurity.HTTP()
			client.HttpClient.Transport = feeBillTransport(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Host != "api.mch.weixin.qq.com" || req.Header.Get("Authorization") == "" {
					t.Fatal("uncontrolled or unsigned request")
				}
				if req.URL.Path == "/v3/bill/downloadurl" {
					downloads++
					if req.URL.RawQuery != "token=original-token" {
						t.Fatal("changed original bill token")
					}
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(file)), Request: req}, nil
				}
				queries++
				if req.URL.Path != "/v3/bill/fundflowbill" || req.URL.Query().Get("account_type") != "FEES" || req.URL.Query().Get("bill_date") != date || len(req.URL.Query()) != 2 {
					t.Fatal("not an exact platform FEES query")
				}
				ts := strconv.FormatInt(now.Unix(), 10)
				digest := sha256.Sum256([]byte(ts + "\nfixture-nonce\n" + string(body) + "\n"))
				signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				if e != nil {
					t.Fatal(e)
				}
				encoded := base64.StdEncoding.EncodeToString(signature)
				if scenario == "invalid_signature" {
					encoded = "invalid"
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"fixture-nonce"}, "Wechatpay-Serial": {cfg.PublicKeyID}, "Wechatpay-Signature": {encoded}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: req}, nil
			})
			p.client.SetHttpClient(client)
			facts, e := p.QueryServiceFees(context.Background(), order, date)
			if queries != 1 {
				t.Fatal("original manifest was not queried exactly once")
			}
			switch scenario {
			case "verified":
				if e != nil || len(facts) != 2 || facts[0].AmountMinor != 3 || facts[0].Returned || facts[1].AmountMinor != 1 || !facts[1].Returned || facts[1].BusinessID != "original-refund" || facts[0].Payment != order.MoneyInput() || !strings.HasPrefix(facts[0].ProofID, "wechat-fees-bill:") {
					t.Fatalf("original fee facts lost: %+v %v", facts, e)
				}
			case "unconfirmed_refund":
				if e != nil || len(facts) != 1 || facts[0].Returned {
					t.Fatalf("unconfirmed refund became a fee fact: %+v %v", facts, e)
				}
			default:
				if e == nil || len(facts) != 0 {
					t.Fatalf("untrusted bill produced fee facts: %+v %v", facts, e)
				}
			}
			wantDownloads := 1
			if scenario == "invalid_signature" || scenario == "external_url" {
				wantDownloads = 0
			}
			if downloads != wantDownloads {
				t.Fatalf("download count %d want %d", downloads, wantDownloads)
			}
			if e != nil && e != b.ErrReconciliationRequired {
				t.Fatalf("untrusted bill lost UNKNOWN classification: %v", e)
			}
		})
	}
}
