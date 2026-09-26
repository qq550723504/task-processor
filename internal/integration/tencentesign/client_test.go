package tencentesign

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	ess "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ess/v20201111"
	domain "task-processor/internal/subjectverification"
	"testing"
	"time"
)

type sdkFunc func(context.Context, *ess.CreateOrganizationAuthUrlRequest) (*ess.CreateOrganizationAuthUrlResponse, error)

func (f sdkFunc) CreateOrganizationAuthUrlWithContext(ctx context.Context, r *ess.CreateOrganizationAuthUrlRequest) (*ess.CreateOrganizationAuthUrlResponse, error) {
	return f(ctx, r)
}
func ptr[T any](v T) *T { return &v }
func TestCreateLocksCompanyAndAuthenticatedPhone(t *testing.T) {
	called := false
	c := &Client{operator: "vendor-operator", sdk: sdkFunc(func(ctx context.Context, r *ess.CreateOrganizationAuthUrlRequest) (*ess.CreateOrganizationAuthUrlResponse, error) {
		called = true
		if *r.AdminMobile != "13800000001" || !*r.AdminMobileSame || !*r.OrganizationNameSame || !*r.UniformSocialCreditCodeSame || *r.Endpoint != "PC" || *r.UserData != "opaque" || *r.Operator.UserId != "vendor-operator" {
			t.Fatal("unbound provider request")
		}
		if len(r.AuthorizationTypes) != 2 || *r.AuthorizationTypes[0] != 2 || *r.AuthorizationTypes[1] != 5 || len(r.Initialization) != 0 || r.BusinessLicense != nil {
			t.Fatal("unexpected authorization or initialization")
		}
		return &ess.CreateOrganizationAuthUrlResponse{Response: &ess.CreateOrganizationAuthUrlResponseParams{AuthUrl: ptr("https://qian.tencent.com/auth"), ExpiredTime: ptr(time.Now().Add(time.Hour).Unix())}}, nil
	})}
	if _, err := c.Create(context.Background(), domain.CreateRequest{CompanyName: "企业", CreditCode: "91310000MA00000001", Phone: "13800000001", Correlation: "opaque"}); err != nil || !called {
		t.Fatalf("create failed: %v", err)
	}
}
func callbackEnvelope(t *testing.T, key, sign []byte, plain []byte) ([]byte, string) {
	t.Helper()
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(encrypted, plain)
	raw, _ := json.Marshal(map[string]string{"encrypt": base64.StdEncoding.EncodeToString(encrypted)})
	mac := hmac.New(sha256.New, sign)
	mac.Write(raw)
	return raw, "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
func TestCallbackAuthenticatesRawBodyBeforeInterpretingIdentity(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	sign := []byte("callback-signing-secret")
	c, err := NewCallback(sign, key)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte(`{"MsgId":"msg-1","MsgType":"CreateOrganization","MsgVersion":"CustomApp","MsgData":{"OrganizationId":"vendor-org","OrganizationName":"企业","UniformSocialCreditCode":"91310000MA00000001","AdminMobile":"13800000001","AdminUserId":"vendor-admin","CreateTime":1790410000,"UserData":"opaque"}}`)
	raw, sig := callbackEnvelope(t, key, sign, plain)
	e, supported, err := c.Parse(raw, sig)
	if err != nil || !supported || e.ProviderAdminID != "vendor-admin" || e.Correlation != "opaque" || e.Phone != "13800000001" {
		t.Fatalf("valid callback: %+v %v", e, err)
	}
	if _, _, err = c.Parse(append(raw, ' '), sig); err == nil {
		t.Fatal("tampered body authenticated")
	}
	if _, _, err = c.Parse(plain, sig); err == nil {
		t.Fatal("unencrypted callback accepted")
	}
	raw, sig = callbackEnvelope(t, key, sign, bytes.Replace(plain, []byte(`"UserData":"opaque"`), []byte(`"UserData":""`), 1))
	if e, supported, err = c.Parse(raw, sig); err != nil || !supported || e.MessageID != "msg-1" || e.Correlation != "" {
		t.Fatal("missing correlation must reach durable replay checks")
	}
	raw, sig = callbackEnvelope(t, key, sign, []byte(`{"MsgId":"msg-2","MsgType":"UserAccountVerify","MsgVersion":"CustomApp","MsgData":{"UserData":"opaque"}}`))
	if _, supported, err = c.Parse(raw, sig); err != nil || supported {
		t.Fatal("personal event treated as enterprise proof")
	}
	raw, sig = callbackEnvelope(t, key, sign, bytes.Replace(plain, []byte(`"CreateTime":1790410000`), []byte(`"CreateTime":0`), 1))
	if _, _, err = c.Parse(raw, sig); err == nil {
		t.Fatal("missing provider timestamp accepted")
	}
}
