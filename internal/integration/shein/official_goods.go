package shein

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	sigjson "sigs.k8s.io/json"
	"strconv"
	"strings"
	"task-processor/internal/integration/httpimage"
	"task-processor/internal/marketplace/shein/goods"
	sheinmodel "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
	"time"
	"unicode/utf8"
)

const MaxGoodsResponseBytes = 2 << 20

var (
	ErrGoodsInvalid        = errors.New("invalid official goods request")
	ErrGoodsUnavailable    = errors.New("official goods information unavailable")
	ErrGoodsOutcomeUnknown = errors.New("official goods mutation outcome unknown")
)

// goodsPost borrows the existing signed transport configuration. It does not
// retry, follow redirects, accept endpoints from callers, or send a fictional
// provider idempotency header. Credential values never enter returned errors.
func (c *OfficialClient) goodsPost(ctx context.Context, path string, credential storecenter.OfficialMerchantCredential, payload []byte) ([]byte, error) {
	return c.goodsRequest(ctx, http.MethodPost, path, "", credential, payload)
}
func (c *OfficialClient) goodsRequest(ctx context.Context, method, path, query string, credential storecenter.OfficialMerchantCredential, payload []byte) ([]byte, error) {
	if ctx == nil || c == nil || credential.AppID != c.application.AppID || !boundedParameter(credential.OpenKeyID, 128) || !boundedParameter(credential.SecretKey, 512) || len(payload) > MaxGoodsResponseBytes {
		return nil, ErrGoodsInvalid
	}
	if _, ok := supplierIdentity(json.RawMessage(credential.SupplierID)); !ok {
		return nil, ErrGoodsInvalid
	}
	switch path {
	case "/open-api/goods/product/publishOrEdit", "/open-api/goods/transform-pic", "/open-api/goods/query-site-list", "/open-api/goods/query-publish-fill-in-standard", "/open-api/goods/query-category-tree", "/open-api/goods/query-attribute-template", "/open-api/goods/get-associated-attribute-rules", "/open-api/goods/query-brand-list":
		if method != http.MethodPost || query != "" {
			return nil, ErrGoodsInvalid
		}
	case "/open-api/goods/product/check-publish-permission":
		if method != http.MethodGet || len(payload) != 0 {
			return nil, ErrGoodsInvalid
		}
	case "/open-api/msc/warehouse/list":
		if method != http.MethodGet || query != "" || len(payload) != 0 {
			return nil, ErrGoodsInvalid
		}
	default:
		return nil, ErrGoodsInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	timestamp := strconv.FormatInt(c.now().UnixMilli(), 10)
	signature, err := c.signature(credential.OpenKeyID, credential.SecretKey, path, timestamp)
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	endpoint := c.origin + path
	if query != "" {
		endpoint += "?" + query
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("language", "en")
	if method == http.MethodGet {
		request.Header.Set("language", "US")
		request.Header.Set("x-lt-language", "US")
	}
	request.Header.Set("x-lt-openKeyId", credential.OpenKeyID)
	request.Header.Set("x-lt-timestamp", timestamp)
	request.Header.Set("x-lt-signature", signature)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ErrGoodsUnavailable
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		media, params, err := mime.ParseMediaType(contentType)
		if err != nil || media != "application/json" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") {
			return nil, ErrGoodsUnavailable
		}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxGoodsResponseBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > MaxGoodsResponseBytes || !utf8.Valid(raw) {
		return nil, ErrGoodsUnavailable
	}
	return raw, nil
}
func decodeGoods(raw []byte, target any) error {
	strict, err := sigjson.UnmarshalStrict(raw, target, sigjson.DisallowDuplicateFields)
	if err != nil || len(strict) > 0 {
		return ErrGoodsUnavailable
	}
	return nil
}
func goodsHash(raw []byte) string { hash := sha256.Sum256(raw); return hex.EncodeToString(hash[:]) }

func (c *OfficialClient) QueryProductSites(ctx context.Context, credential storecenter.OfficialMerchantCredential) ([]sheinmodel.MainSite, error) {
	raw, err := c.goodsPost(ctx, "/open-api/goods/query-site-list", credential, nil)
	if err != nil {
		return nil, ErrGoodsUnavailable
	}
	var response struct {
		Code string `json:"code"`
		Info *struct {
			Data []sheinmodel.MainSite `json:"data"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.Data == nil || len(response.Info.Data) > 1000 {
		return nil, ErrGoodsUnavailable
	}
	for _, main := range response.Info.Data {
		if !boundedParameter(main.ID, 128) || len(main.Sites) > 1000 {
			return nil, ErrGoodsUnavailable
		}
		for _, site := range main.Sites {
			if !boundedParameter(site.Abbreviation, 128) || site.Status != nil && *site.Status != 0 && *site.Status != 1 {
				return nil, ErrGoodsUnavailable
			}
		}
	}
	return response.Info.Data, nil
}
func (c *OfficialClient) QueryProductFillStandards(ctx context.Context, credential storecenter.OfficialMerchantCredential, categoryID int64) (sheinmodel.FillStandards, error) {
	if categoryID <= 0 {
		return sheinmodel.FillStandards{}, ErrGoodsInvalid
	}
	payload, _ := json.Marshal(struct {
		CategoryID int64 `json:"category_id"`
	}{categoryID})
	raw, err := c.goodsPost(ctx, "/open-api/goods/query-publish-fill-in-standard", credential, payload)
	if err != nil {
		return sheinmodel.FillStandards{}, ErrGoodsUnavailable
	}
	var response struct {
		Code string                    `json:"code"`
		Info *sheinmodel.FillStandards `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || !boundedParameter(response.Info.DefaultLanguage, 32) || response.Info.Fields == nil || len(response.Info.Fields) > 256 || len(response.Info.Pictures) > 64 || len(response.Info.TitleLimits) > 256 {
		return sheinmodel.FillStandards{}, ErrGoodsUnavailable
	}
	fields := map[string]bool{}
	for _, field := range response.Info.Fields {
		if !boundedParameter(field.Field, 128) || field.Required == nil || field.Show == nil || fields[field.Field] {
			return sheinmodel.FillStandards{}, ErrGoodsUnavailable
		}
		fields[field.Field] = true
	}
	pictures := map[string]bool{}
	for _, rule := range response.Info.Pictures {
		if !boundedParameter(rule.Field, 128) || rule.Enabled == nil || pictures[rule.Field] {
			return sheinmodel.FillStandards{}, ErrGoodsUnavailable
		}
		pictures[rule.Field] = true
	}
	for _, limit := range response.Info.TitleLimits {
		if !boundedParameter(limit.Language, 32) || limit.Maximum < 2 || limit.Maximum > 5000 || math.Trunc(limit.Maximum) != limit.Maximum {
			return sheinmodel.FillStandards{}, ErrGoodsUnavailable
		}
	}
	return *response.Info, nil
}

// A successful HTTP or code=0 response alone cannot authorize "uploaded".
// This new-product response must contain exactly the requested SKU membership,
// unique platform identifiers and the same SKC grouping as the sent payload.
func (c *OfficialClient) PublishProduct(ctx context.Context, credential storecenter.OfficialMerchantCredential, input sheinmodel.PublishProduct) (sheinmodel.PublishResult, error) {
	if input.CategoryID <= 0 || input.ProductTypeID <= 0 || input.SourceSystem != "OpenAPI" || input.SuitFlag != "0" || len(input.SKCs) < 1 || len(input.SKCs) > 40 || len(input.Names) == 0 || len(input.Sites) != 1 || input.Sites[0].MainSite != "shein" || len(input.Sites[0].SubSites) != 1 || input.Sites[0].SubSites[0] != "shein-us" {
		return sheinmodel.PublishResult{}, ErrGoodsInvalid
	}
	expected := map[string]int{}
	for index, skc := range input.SKCs {
		if len(skc.SKUs) < 1 || len(skc.SKUs) > 400 {
			return sheinmodel.PublishResult{}, ErrGoodsInvalid
		}
		for _, sku := range skc.SKUs {
			if !boundedParameter(sku.SupplierSKU, 800) {
				return sheinmodel.PublishResult{}, ErrGoodsInvalid
			}
			if _, exists := expected[sku.SupplierSKU]; exists {
				return sheinmodel.PublishResult{}, ErrGoodsInvalid
			}
			expected[sku.SupplierSKU] = index
		}
	}
	payload, err := json.Marshal(input)
	if err != nil || len(payload) > MaxGoodsResponseBytes {
		return sheinmodel.PublishResult{}, ErrGoodsInvalid
	}
	raw, err := c.goodsPost(ctx, "/open-api/goods/product/publishOrEdit", credential, payload)
	if err != nil {
		return sheinmodel.PublishResult{}, ErrGoodsOutcomeUnknown
	}
	var response struct {
		Code    string `json:"code"`
		TraceID string `json:"traceId"`
		Info    *struct {
			Success    *bool                     `json:"success"`
			SPUName    string                    `json:"spu_name"`
			SKCs       []sheinmodel.PublishedSKC `json:"skc_list"`
			Version    string                    `json:"version"`
			MCCResults []struct {
				Type *int `json:"type"`
			} `json:"mcc_valid_result"`
			FilteredResults      []json.RawMessage `json:"filtered_result"`
			PreValidationResults []json.RawMessage `json:"pre_valid_result"`
		} `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.Success == nil || !*response.Info.Success || !boundedParameter(response.Info.SPUName, 128) || len(response.Info.SKCs) != len(input.SKCs) {
		return sheinmodel.PublishResult{}, ErrGoodsOutcomeUnknown
	}
	if len(response.Info.FilteredResults) > 0 || len(response.Info.PreValidationResults) > 0 {
		return sheinmodel.PublishResult{}, ErrGoodsOutcomeUnknown
	}
	for _, result := range response.Info.MCCResults {
		if result.Type == nil || *result.Type != 1 {
			return sheinmodel.PublishResult{}, ErrGoodsOutcomeUnknown
		}
	}
	result := sheinmodel.PublishResult{SPUName: response.Info.SPUName, SKCs: response.Info.SKCs, Version: response.Info.Version, TraceID: response.TraceID, ResponseHash: goodsHash(raw)}
	if !goods.CorrelatesPublishedMembership(input, result) {
		return sheinmodel.PublishResult{}, ErrGoodsOutcomeUnknown
	}
	return result, nil
}

func (c *OfficialClient) TransformProductImage(ctx context.Context, credential storecenter.OfficialMerchantCredential, input sheinmodel.TransformImage) (sheinmodel.TransformedImage, error) {
	if !canonicalPublicImage(input.OriginalURL) || input.Type != 1 && input.Type != 2 && input.Type != 5 && input.Type != 6 && input.Type != 7 {
		return sheinmodel.TransformedImage{}, ErrGoodsInvalid
	}
	payload, _ := json.Marshal(input)
	raw, err := c.goodsPost(ctx, "/open-api/goods/transform-pic", credential, payload)
	if err != nil {
		return sheinmodel.TransformedImage{}, ErrGoodsOutcomeUnknown
	}
	var response struct {
		Code string                       `json:"code"`
		Info *sheinmodel.TransformedImage `json:"info"`
	}
	if decodeGoods(raw, &response) != nil || response.Code != "0" || response.Info == nil || response.Info.Original != input.OriginalURL || response.Info.FailureReason != "" || !isOfficialImage(response.Info.Transformed) {
		return sheinmodel.TransformedImage{}, ErrGoodsOutcomeUnknown
	}
	response.Info.ResponseHash = goodsHash(raw)
	return *response.Info, nil
}
func canonicalPublicImage(value string) bool {
	validated, err := httpimage.ValidatePublicHTTPSURL(value)
	parsed, parseErr := url.Parse(value)
	return err == nil && parseErr == nil && validated == value && len(value) <= 2048 && parsed.User == nil && parsed.Fragment == "" && (parsed.Port() == "" || parsed.Port() == "443")
}
func isOfficialImage(value string) bool {
	if !canonicalPublicImage(value) {
		return false
	}
	parsed, _ := url.Parse(value)
	host := strings.ToLower(parsed.Hostname())
	return host == "shein.com" || strings.HasSuffix(host, ".shein.com") || host == "ltwebstatic.com" || strings.HasSuffix(host, ".ltwebstatic.com")
}
