package shein

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"task-processor/internal/storecenter"
)

const exchangePath = "/open-api/auth/get-by-token"
const storeInformationPath = "/open-api/openapi-business-backend/query-store-info"

type OfficialClientOptions struct {
	Application storecenter.OfficialApplication
	AppSecret   string
	APIOrigin   string
	HTTPClient  *http.Client
}
type OfficialClient struct {
	application       storecenter.OfficialApplication
	appSecret, origin string
	http              *http.Client
	now               func() time.Time
	random            io.Reader
}

func NewOfficialClient(options OfficialClientOptions) (*OfficialClient, error) {
	callback, err := url.Parse(options.Application.CallbackURL)
	if err != nil || callback.Scheme != "https" || callback.Host == "" || callback.User != nil || callback.Fragment != "" || callback.RawQuery != "" || !boundedParameter(options.Application.AppID, 128) || !boundedParameter(options.Application.Version, 128) || !boundedParameter(options.AppSecret, 512) || len([]byte(options.AppSecret)) < 16 || options.APIOrigin != "https://openapi.sheincorp.com" && options.APIOrigin != "https://openapi.sheincorp.cn" {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	client := http.Client{Timeout: 5 * time.Second}
	if options.HTTPClient != nil {
		client = *options.HTTPClient
	}
	if client.Timeout <= 0 || client.Timeout > 5*time.Second {
		client.Timeout = 5 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &OfficialClient{application: options.Application, appSecret: options.AppSecret, origin: options.APIOrigin, http: &client, now: time.Now, random: rand.Reader}, nil
}

var _ storecenter.OfficialConnectionProvider = (*OfficialClient)(nil)

func boundedParameter(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
}
func (c *OfficialClient) Application() storecenter.OfficialApplication { return c.application }
func (c *OfficialClient) AuthorizationURL(state string) (string, error) {
	if !boundedParameter(state, 256) {
		return "", storecenter.ErrOfficialConnectionUnavailable
	}
	query := url.Values{"appid": []string{c.application.AppID}, "redirectUrl": []string{base64.StdEncoding.EncodeToString([]byte(c.application.CallbackURL))}, "state": []string{state}}
	return "https://openapi-sem.sheincorp.com/#/empower?" + query.Encode(), nil
}
func (c *OfficialClient) signature(identity, secret, path, timestamp string) (string, error) {
	random := make([]byte, 3)
	if _, err := io.ReadFull(c.random, random); err != nil {
		return "", storecenter.ErrOfficialConnectionUnavailable
	}
	prefix := hex.EncodeToString(random)[:5]
	mac := hmac.New(sha256.New, []byte(secret+prefix))
	_, _ = mac.Write([]byte(identity + "&" + timestamp + "&" + path))
	return prefix + base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(mac.Sum(nil)))), nil
}
func (c *OfficialClient) post(ctx context.Context, path, header, identity, secret string, payload []byte) ([]byte, error) {
	timestamp := strconv.FormatInt(c.now().UnixMilli(), 10)
	signature, err := c.signature(identity, secret, path, timestamp)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.origin+path, bytes.NewReader(payload))
	if err != nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set(header, identity)
	request.Header.Set("x-lt-timestamp", timestamp)
	request.Header.Set("x-lt-signature", signature)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
	if err != nil || len(data) > 64*1024 {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return data, nil
}

func (c *OfficialClient) Exchange(ctx context.Context, token, state string) (storecenter.OfficialMerchantCredential, error) {
	if !boundedParameter(token, 2048) || !boundedParameter(state, 256) {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialAuthorizationRejected
	}
	payload, _ := json.Marshal(struct {
		Token string `json:"tempToken"`
	}{token})
	data, err := c.post(ctx, exchangePath, "x-lt-appid", c.application.AppID, c.appSecret, payload)
	if err != nil {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialExchangeUnknown
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			SecretKey  string          `json:"secretKey"`
			OpenKeyID  string          `json:"openKeyId"`
			AppID      string          `json:"appid"`
			State      string          `json:"state"`
			SupplierID json.RawMessage `json:"supplierId"`
		} `json:"info"`
	}
	if json.Unmarshal(data, &response) != nil || response.Code == "" {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialExchangeUnknown
	}
	if response.Code != "0" {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialAuthorizationRejected
	}
	info := response.Info
	if info == nil || info.AppID != c.application.AppID || !hmac.Equal([]byte(info.State), []byte(state)) || !boundedParameter(info.OpenKeyID, 128) {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialExchangeUnknown
	}
	supplier, ok := supplierIdentity(info.SupplierID)
	if !ok {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialExchangeUnknown
	}
	secret, err := decryptMerchantSecret(info.SecretKey, c.appSecret)
	if err != nil {
		return storecenter.OfficialMerchantCredential{}, storecenter.ErrOfficialExchangeUnknown
	}
	return storecenter.OfficialMerchantCredential{AppID: info.AppID, OpenKeyID: info.OpenKeyID, SecretKey: secret, SupplierID: supplier}, nil
}
func supplierIdentity(value json.RawMessage) (string, bool) {
	text := string(value)
	if text == "" || text[0] < '1' || text[0] > '9' || len(text) > 19 {
		return "", false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	_, err := strconv.ParseInt(text, 10, 64)
	return text, err == nil
}
func decryptMerchantSecret(encoded, appSecret string) (string, error) {
	if len(encoded) > 8192 {
		return "", storecenter.ErrOfficialExchangeUnknown
	}
	sealed, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(sealed) == 0 || len(sealed)%aes.BlockSize != 0 {
		return "", storecenter.ErrOfficialExchangeUnknown
	}
	block, err := aes.NewCipher([]byte(appSecret)[:16])
	if err != nil {
		return "", storecenter.ErrOfficialExchangeUnknown
	}
	plain := make([]byte, len(sealed))
	cipher.NewCBCDecrypter(block, []byte("space-station-default-iv")[:16]).CryptBlocks(plain, sealed)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(plain) {
		return "", storecenter.ErrOfficialExchangeUnknown
	}
	for _, v := range plain[len(plain)-padding:] {
		if int(v) != padding {
			return "", storecenter.ErrOfficialExchangeUnknown
		}
	}
	secret := string(plain[:len(plain)-padding])
	if !boundedParameter(secret, 512) {
		return "", storecenter.ErrOfficialExchangeUnknown
	}
	return secret, nil
}
func (c *OfficialClient) QueryStore(ctx context.Context, credential storecenter.OfficialMerchantCredential) (storecenter.OfficialStoreInformation, error) {
	if credential.AppID != c.application.AppID || !boundedParameter(credential.OpenKeyID, 128) || !boundedParameter(credential.SecretKey, 512) {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	if _, ok := supplierIdentity(json.RawMessage(credential.SupplierID)); !ok {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	data, err := c.post(ctx, storeInformationPath, "x-lt-openKeyId", credential.OpenKeyID, credential.SecretKey, []byte("{}"))
	if err != nil {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			StoreInfo *struct {
				SupplierID  json.RawMessage `json:"supplierId"`
				StoreName   *string         `json:"storeName"`
				StoreStatus *int            `json:"storeStatus"`
			} `json:"storeInfo"`
		} `json:"info"`
	}
	if json.Unmarshal(data, &response) != nil {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	if response.Code == "openapi00004" {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialCredentialExpired
	}
	if response.Code != "0" || response.Info == nil {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	info := storecenter.OfficialStoreInformation{}
	if store := response.Info.StoreInfo; store != nil {
		if len(store.SupplierID) > 0 && string(store.SupplierID) != "null" {
			supplier, ok := supplierIdentity(store.SupplierID)
			if !ok || supplier != credential.SupplierID {
				return info, storecenter.ErrOfficialConnectionUnavailable
			}
			info.SupplierID = &supplier
		}
		if store.StoreName != nil && len(*store.StoreName) > 1000 || store.StoreStatus != nil && *store.StoreStatus != 0 && *store.StoreStatus != 1 {
			return info, storecenter.ErrOfficialConnectionUnavailable
		}
		info.StoreName = store.StoreName
		info.StoreStatus = store.StoreStatus
	}
	return info, nil
}
