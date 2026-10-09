package shein

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/marketplace/shein/goods"
	sheinmodel "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
	"testing"
)

func goodsCredential() storecenter.OfficialMerchantCredential {
	return storecenter.OfficialMerchantCredential{AppID: "test-app", OpenKeyID: "open-key", SecretKey: "merchant-secret", SupplierID: "123"}
}
func goodsProduct() sheinmodel.PublishProduct {
	return sheinmodel.PublishProduct{CategoryID: 123, ProductTypeID: 456, SourceSystem: "OpenAPI", SuitFlag: "0", Names: []sheinmodel.LanguageContent{{Language: "en", Name: "Real source product"}}, Attributes: []sheinmodel.AttributeValue{}, Sites: []sheinmodel.SiteSelection{{MainSite: "shein", SubSites: []string{"shein-us"}}}, SKCs: []sheinmodel.ProductSKC{{SupplierCode: "merchant-product", SaleAttribute: sheinmodel.AttributeValue{AttributeID: 12, AttributeValueID: func() *int64 { v := int64(34); return &v }()}, ImageInfo: sheinmodel.ImageInfo{Images: []sheinmodel.ProductImage{{Sort: 1, Type: 1, URL: "https://img.shein.com/source.jpg"}}}, SKUs: []sheinmodel.ProductSKU{{SupplierSKU: "sku-a", MallState: 1, Prices: []sheinmodel.ProductPrice{{BasePrice: 12.5, Currency: "USD", SubSite: "shein-us"}}, Stock: []sheinmodel.ProductStock{{Quantity: 3}}, SaleAttributes: []sheinmodel.AttributeValue{}}}}}}
}

func TestOfficialPublishAcceptsManagedSiteLessPayloadAndRejectsOtherSites(t *testing.T) {
	calls := 0
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "/open-api/goods/product/publishOrEdit", r.URL.Path)
		require.NotEmpty(t, r.Header.Get("x-lt-signature"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "site_list")
		require.Contains(t, string(raw), "cost_info")
		response := `{"code":"0","info":{"success":true,"spu_name":"spu-managed","skc_list":[{"skc_name":"skc-managed","sku_list":[{"sku_code":"remote-sku","supplier_sku":"sku-a"}]}]}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	}))
	product := goodsProduct()
	product.Sites = nil
	product.SKCs[0].SKUs[0].Prices = nil
	product.SKCs[0].SKUs[0].Cost = &sheinmodel.CostPrice{Price: "10.50", Currency: "CNY"}
	result, err := client.PublishProduct(context.Background(), goodsCredential(), product)
	require.NoError(t, err)
	require.Equal(t, "spu-managed", result.SPUName)
	for _, sites := range [][]sheinmodel.SiteSelection{
		{{MainSite: "shein", SubSites: []string{"shein-ca"}}},
		{{MainSite: "other", SubSites: []string{"shein-us"}}},
		{{MainSite: "shein", SubSites: []string{"shein-us", "shein-ca"}}},
		{{MainSite: "shein", SubSites: []string{"shein-us"}}, {MainSite: "shein", SubSites: []string{"shein-us"}}},
	} {
		product.Sites = sites
		_, err = client.PublishProduct(context.Background(), goodsCredential(), product)
		require.ErrorIs(t, err, ErrGoodsInvalid)
	}
	require.Equal(t, 1, calls, "invalid sites never reach transport and the mutation is never retried")
}

func TestOfficialSPUReadbackUsesSignedReadOnlyContractAndNeverProvesAbsence(t *testing.T) {
	response := `{"code":"0","info":{"spuName":"spu-real","categoryId":123,"productTypeId":456,"productMultiNameList":[{"language":"en","productName":"Real source product"}],"skcInfoList":[{"skcName":"skc-real","supplierCode":"merchant-product","skuInfoList":[{"skuCode":"remote-sku","supplierSku":"sku-a"}]}],"futureField":true}}`
	calls := 0
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "https://openapi.sheincorp.com/open-api/goods/spu-info", r.URL.String())
		require.NotEmpty(t, r.Header.Get("x-lt-signature"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"spuName":"spu-real","languageList":["en"]}`, string(raw))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	}))
	v, err := client.QueryProductSPU(context.Background(), goodsCredential(), "spu-real")
	require.NoError(t, err)
	require.True(t, goods.CorrelatesProductReadback(goodsProduct(), "spu-real", v))
	require.Equal(t, goodsHash([]byte(response)), v.Product.ResponseHash)
	for _, bad := range []string{`{"code":"0003","info":null}`, `{"code":"0","info":{"spuName":"another"}}`, `{"code":"0","code":"0","info":null}`} {
		response = bad
		_, err = client.QueryProductSPU(context.Background(), goodsCredential(), "spu-real")
		require.ErrorIs(t, err, ErrGoodsUnavailable)
	}
	require.Equal(t, 4, calls, "each read is single-shot and never publishes")
}

func TestOfficialGoodsPublishSignsCurrentAPIAndCorrelatesEveryReturnedSKU(t *testing.T) {
	response := `{"code":"0","info":{"success":true,"spu_name":"spu-real","skc_list":[{"skc_name":"skc-real","sku_list":[{"sku_code":"remote-sku","supplier_sku":"sku-a"}]}],"version":"1"},"traceId":"trace-real"}`
	calls := 0
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https://openapi.sheincorp.com/open-api/goods/product/publishOrEdit", r.URL.String())
		require.Equal(t, "open-key", r.Header.Get("x-lt-openKeyId"))
		require.Equal(t, "en", r.Header.Get("language"))
		require.Empty(t, r.Header.Get("Idempotency-Key"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var body map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &body))
		require.Contains(t, body, "site_list")
		require.NotContains(t, body, "productsite_list")
		require.NotContains(t, body, "spu_name")
		require.NotContains(t, body, "extra")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	}))
	result, err := client.PublishProduct(context.Background(), goodsCredential(), goodsProduct())
	require.NoError(t, err)
	require.Equal(t, "spu-real", result.SPUName)
	require.Equal(t, "remote-sku", result.SKCs[0].SKUs[0].SKUCode)
	require.Len(t, result.ResponseHash, 64)
	require.Equal(t, 1, calls)
	for _, invalid := range []string{
		`{"code":"0","info":{"success":true}}`,
		`{"code":"0","info":{"success":false}}`,
		`{"code":"0","code":"error","info":{"success":true}}`,
		strings.Replace(response, `"sku-a"`, `"unrelated-sku"`, 1),
		strings.Replace(response, `"skc-real"`, `""`, 1),
		strings.Replace(response, `"success":true`, `"success":true,"mcc_valid_result":[{"type":2,"message":"blocked"}]`, 1),
		strings.Replace(response, `"success":true`, `"success":true,"filtered_result":[{"scene":"attributes","message":"removed"}]`, 1),
		`{"code":"unrecognized","msg":"private provider details"}`,
	} {
		response = invalid
		_, err = client.PublishProduct(context.Background(), goodsCredential(), goodsProduct())
		require.ErrorIs(t, err, ErrGoodsOutcomeUnknown)
		require.NotContains(t, err.Error(), "private provider details")
	}
	require.Equal(t, 9, calls, "the transport never retries a mutation")
}

func TestOfficialImageConversionRequiresExactSourceAndActualSHEINReference(t *testing.T) {
	response := `{"code":"0","info":{"original":"https://images.example.org/source.jpg","transformed":"https://img.shein.com/converted.jpg","failure_reason":""}}`
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/open-api/goods/transform-pic", r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"original_url":"https://images.example.org/source.jpg","image_type":5}`, string(raw))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	input := sheinmodel.TransformImage{OriginalURL: "https://images.example.org/source.jpg", Type: 5}
	converted, err := client.TransformProductImage(context.Background(), goodsCredential(), input)
	require.NoError(t, err)
	require.Equal(t, "https://img.shein.com/converted.jpg", converted.Transformed)
	for _, invalid := range []string{
		`{"code":"0","info":{"original":"https://images.example.org/source.jpg","transformed":"","failure_reason":"图片下载异常"}}`,
		strings.Replace(response, "source.jpg", "replacement.jpg", 1),
		strings.Replace(response, "https://img.shein.com/converted.jpg", "https://attacker.example/stolen.jpg", 1),
	} {
		response = invalid
		_, err = client.TransformProductImage(context.Background(), goodsCredential(), input)
		require.ErrorIs(t, err, ErrGoodsOutcomeUnknown)
	}
	input.OriginalURL = "https://127.0.0.1/private.jpg"
	_, err = client.TransformProductImage(context.Background(), goodsCredential(), input)
	require.ErrorIs(t, err, ErrGoodsInvalid)
}

func TestOfficialGoodsQueriesPreserveUnknownEnablementAndRequiredFlags(t *testing.T) {
	response := `{"code":"0","info":{"data":[{"main_site":"shein","sub_site_list":[{"site_abbr":"shein-us","currency":"USD"}]}]}}`
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	sites, err := client.QueryProductSites(context.Background(), goodsCredential())
	require.NoError(t, err)
	require.Nil(t, sites[0].Sites[0].Status)
	response = `{"code":"0","info":{"default_language":"en","fill_in_standard_list":[{"field_key":"brand_code","show":true,"required":false}],"picture_config_list":[{"field_key":"sku_image_required","is_true":true}],"supplier_code_in_spu_dimension":true,"default_language_title_max_length":150}}`
	standards, err := client.QueryProductFillStandards(context.Background(), goodsCredential(), 123)
	require.NoError(t, err)
	require.NotNil(t, standards.SupplierCodeInSPU)
	require.True(t, *standards.Pictures[0].Enabled)
	response = `{"code":"0","info":{"default_language":"en","fill_in_standard_list":[{"field_key":"brand_code","show":true}]}}`
	_, err = client.QueryProductFillStandards(context.Background(), goodsCredential(), 123)
	require.ErrorIs(t, err, ErrGoodsUnavailable, "missing required=false is unknown, not optional")
	response = strings.Repeat("x", MaxGoodsResponseBytes+1)
	_, err = client.QueryProductSites(context.Background(), goodsCredential())
	require.ErrorIs(t, err, ErrGoodsUnavailable)
	broken := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("private network detail") }))
	_, err = broken.PublishProduct(context.Background(), goodsCredential(), goodsProduct())
	require.ErrorIs(t, err, ErrGoodsOutcomeUnknown)
	require.NotContains(t, err.Error(), "private network")
}
