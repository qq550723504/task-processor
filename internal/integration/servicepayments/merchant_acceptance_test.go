package servicepayments

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strconv"
	"strings"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/paymentsecurity"
	"testing"
	"time"
)

func TestMerchantAcceptanceComesFromActualCurrentRevisionSDKSigned200(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture, _, _ := wechatServiceFixture()
	cfg := fixture.config
	cfg.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	cfg.PublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
	cfg.SerialNumber, cfg.APIv3Key, cfg.NotifyURL = "fixture-serial", strings.Repeat("a", 32), "https://platform.example/notify"
	cfg.NewMerchantApplications, cfg.ProductQualified = true, true
	now := time.Now().UTC()
	for _, scenario := range []string{"current-same-applyment", "wrong-number", "invalid-signature", "unsigned", "not-200", "duplicate-response-field"} {
		t.Run(scenario, func(t *testing.T) {
			p, err := NewWeChat(cfg)
			if err != nil {
				t.Fatal(err)
			}
			p.now = func() time.Time { return now }
			ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
			root := e.MerchantIntent{ID: uuid.NewString(), ApplicationID: uuid.NewString(), OrganizationID: "original-org", ActorID: "actor", Key: uuid.NewString(), ApplicationVersion: 4, OutRequestNo: "original-correction-number", Profile: p.MerchantProfile(), FileIDs: ids, LicenseFileID: ids[0], Fingerprint: "immutable-root", SealedDetails: []byte("immutable-original-bank")}
			d := e.MerchantDetails{ExpectedRevisionVersion: 1, LicenseFileID: ids[0], Legal: e.IdentityDocument{Type: "IDENTIFICATION_TYPE_MAINLAND_IDCARD", Name: "legal", Number: "legal-number", FrontFileID: ids[1], BackFileID: ids[2], ValidFrom: "2020-01-01", ValidUntil: "长期"}, SoleLegalBeneficiary: true, ContactMobile: "13800138000", AccountBank: "工商银行", AccountNumber: "corrected-private-bank", MerchantShortName: "short", StoreName: "store", StoreURL: "https://provider.example/"}
			input := root
			input.ID = uuid.NewString()
			input.Key = uuid.NewString()
			input.ApplicationVersion = 8
			input.ExpectedRevisionVersion = 1
			input.Fingerprint = e.Fingerprint([]any{e.Scope{OrganizationID: input.OrganizationID, ActorID: input.ActorID}, input.Key, input.ApplicationID, input.ApplicationVersion, input.Profile, d})
			protection, err := NewMerchantPayloadProtection([]byte(strings.Repeat("s", 32)))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(d)
			input.SealedDetails, err = protection.Seal(input.ID, string(raw))
			if err != nil {
				t.Fatal(err)
			}
			opened, err := protection.Open(input.ID, input.SealedDetails)
			if err != nil || json.Unmarshal([]byte(opened), &d) != nil {
				t.Fatal(err)
			}
			a := e.MerchantAttempt{Intent: root, Revision: e.MerchantDetailsRevision{ID: input.ID, Version: 2, Input: input}, Dispatched: true, CompanyName: "approved-company", RegistrationNumber: "approved-registration", MediaIDs: map[string]string{ids[0]: "license-media", ids[1]: "front-media", ids[2]: "back-media"}}
			response := `{"applyment_id":1001,"out_request_no":"original-correction-number"}`
			if scenario == "wrong-number" {
				response = `{"applyment_id":1001,"out_request_no":"foreign"}`
			}
			if scenario == "duplicate-response-field" {
				response = `{"applyment_id":1001,"out_request_no":"foreign","out_request_no":"original-correction-number"}`
			}
			requests := 0
			client := paymentsecurity.HTTP()
			client.HttpClient.Transport = feeBillTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v3/certificates" {
					return nil, e.ErrUnavailable
				}
				requests++
				if req.URL.Host != "api.mch.weixin.qq.com" || req.Method != http.MethodPost || req.URL.Path != "/v3/ecommerce/applyments/" || req.Header.Get("Authorization") == "" {
					t.Fatal("not actual signed merchant SDK submission")
				}
				var sent map[string]any
				if json.NewDecoder(req.Body).Decode(&sent) != nil || sent["out_request_no"] != root.OutRequestNo {
					t.Fatal("original number changed")
				}
				account := sent["account_info"].(map[string]any)
				decrypted, err := p.client.V3DecryptText(account["account_number"].(string))
				if err != nil || decrypted != d.AccountNumber {
					t.Fatal("SDK did not send current sealed correction", err)
				}
				ts := strconv.FormatInt(now.Unix(), 10)
				digest := sha256.Sum256([]byte(ts + "\nmerchant-proof\n" + response + "\n"))
				signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
				if err != nil {
					t.Fatal(err)
				}
				header := http.Header{"Wechatpay-Timestamp": {ts}, "Wechatpay-Nonce": {"merchant-proof"}, "Wechatpay-Serial": {cfg.PublicKeyID}, "Wechatpay-Signature": {base64.StdEncoding.EncodeToString(signature)}}
				if scenario == "invalid-signature" {
					header.Set("Wechatpay-Signature", "invalid")
				}
				if scenario == "unsigned" {
					header = http.Header{}
				}
				status := http.StatusOK
				if scenario == "not-200" {
					status = http.StatusAccepted
				}
				return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(response)), Request: req}, nil
			})
			p.client.SetHttpClient(client)
			proof, err := p.SubmitMerchant(context.Background(), a, d)
			if scenario == "current-same-applyment" {
				if err != nil || !proof.Matches(a) || proof.ChannelApplicationID != "1001" || proof.SignedResponseDigest != e.FileDigest([]byte(response)) || proof.DetailsFingerprint == root.Fingerprint {
					t.Fatal("current signed submission proof missing", proof, err)
				}
			} else if err == nil || proof.RevisionID != "" {
				t.Fatal("untrusted response fabricated acceptance", proof, err)
			}
			if requests != 1 {
				t.Fatal("unexpected submission retry", requests)
			}
		})
	}
}
