package shein

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	sheinmodel "task-processor/internal/marketplace/shein/model"
	"testing"
)

func TestOfficialGoodsRulesUseMerchantEndpointsAndExactTypedRequests(t *testing.T) {
	path, payload, response := "/open-api/goods/query-category-tree", "", `{"code":"0","info":{"data":[{"category_id":123,"category_name":"Leaf","last_category":true,"product_type_id":456,"children":[]}]}}`
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, path, r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if payload == "" {
			require.Empty(t, raw)
		} else {
			require.JSONEq(t, payload, string(raw))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	ctx := context.Background()
	categories, err := client.QueryProductCategories(ctx, goodsCredential())
	require.NoError(t, err)
	require.True(t, *categories[0].Leaf)
	require.EqualValues(t, 456, categories[0].ProductTypeID)
	path, payload, response = "/open-api/goods/query-attribute-template", `{"product_type_id_list":[456]}`, `{"code":"0","info":{"data":[{"product_type_id":456,"main_attribute_status":1,"attribute_infos":[{"attribute_id":12,"attribute_name":"Color","attribute_is_show":1,"attribute_type":1,"attribute_label":1,"attribute_mode":2,"attribute_input_num":1,"attribute_status":3,"value_info_list":[{"attribute_value_id":34,"attribute_value":"Blue","is_show":1}]}]}]}}`
	attributes, err := client.QueryProductAttributes(ctx, goodsCredential(), 456)
	require.NoError(t, err)
	require.EqualValues(t, 12, attributes.Attributes[0].ID)
	require.Equal(t, 2, *attributes.Attributes[0].Mode)
	path, payload, response = "/open-api/goods/get-associated-attribute-rules", `{"get_linked_rule_req_list":[{"group_id":"source-1","category_id":123,"product_type_id":456,"attribute_list":[{"attribute_id":56,"attribute_value_id":78}]}]}`, `{"code":"0","info":{"data":[{"group_id":"source-1","link_rule_attribute_list":[{"attribute_id":90,"attribute_value_list":[91],"attribute_value_pre_fill_list":[92]}]}]}}`
	value := int64(78)
	input := sheinmodel.LinkedRulesRequest{Groups: []sheinmodel.LinkedRuleGroup{{ID: "source-1", CategoryID: 123, ProductTypeID: 456, Attributes: []sheinmodel.AttributeValue{{AttributeID: 56, AttributeValueID: &value}}}}}
	linked, err := client.QueryProductLinkedRules(ctx, goodsCredential(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{92}, linked[0].Attributes[0].PrefilledValues)
	response = strings.Replace(response, "source-1", "foreign-group", 1)
	_, err = client.QueryProductLinkedRules(ctx, goodsCredential(), input)
	require.ErrorIs(t, err, ErrGoodsUnavailable)
	path, payload, response = "/open-api/goods/query-brand-list", "", `{"code":"0","info":{"data":[{"brand_code":"actual-brand","brand_name":"Merchant brand"}]}}`
	brands, err := client.QueryProductBrands(ctx, goodsCredential())
	require.NoError(t, err)
	require.Equal(t, "actual-brand", brands[0].Code)
}

func TestOfficialGoodsRulesRejectIncompleteAndUncorrelatedFacts(t *testing.T) {
	response := `{"code":"0","info":{"data":[{"category_id":123,"last_category":true}]}}`
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	_, err := client.QueryProductCategories(context.Background(), goodsCredential())
	require.ErrorIs(t, err, ErrGoodsUnavailable)
	response = `{"code":"0","info":{"data":[{"product_type_id":999,"main_attribute_status":1,"attribute_infos":[]}]}}`
	_, err = client.QueryProductAttributes(context.Background(), goodsCredential(), 456)
	require.ErrorIs(t, err, ErrGoodsUnavailable)
	response = `{"code":"0","info":{"data":null}}`
	_, err = client.QueryProductBrands(context.Background(), goodsCredential())
	require.ErrorIs(t, err, ErrGoodsUnavailable)
}

func TestPublishPermissionIsCurrentSignedReadAndMissingFlagNeverAllowsSend(t *testing.T) {
	response := `{"code":"0","info":{"canPublishProduct":true,"reason":null}}`
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/open-api/goods/product/check-publish-permission", r.URL.Path)
		require.Equal(t, "actual brand", r.URL.Query().Get("brandCode"))
		require.Equal(t, "US", r.Header.Get("language"))
		require.NotEmpty(t, r.Header.Get("x-lt-signature"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	permission, err := client.QueryProductPublishPermission(context.Background(), goodsCredential(), "actual brand")
	require.NoError(t, err)
	require.True(t, permission.Allowed)
	response = `{"code":"0","info":{"canPublishProduct":false,"reason":"KYC未完成"}}`
	permission, err = client.QueryProductPublishPermission(context.Background(), goodsCredential(), "actual brand")
	require.NoError(t, err)
	require.False(t, permission.Allowed)
	require.Equal(t, "KYC未完成", permission.Reason)
	response = `{"code":"0","info":{"reason":null}}`
	_, err = client.QueryProductPublishPermission(context.Background(), goodsCredential(), "actual brand")
	require.ErrorIs(t, err, ErrGoodsUnavailable)
}

func TestWarehouseReadUsesSignedMerchantGetAndValidatesFullSet(t *testing.T) {
	response := `{"code":"0","info":{"list":[{"warehouseCode":"wh-us","warehouseName":"US","warehouseType":1,"saleCountryList":["US"]},{"warehouseCode":"certified","warehouseName":"Certified","warehouseType":"2","saleCountryList":["US"]}]}}`
	client := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/open-api/msc/warehouse/list", r.URL.Path)
		require.Empty(t, r.URL.RawQuery)
		require.Equal(t, "US", r.Header.Get("x-lt-language"))
		require.NotEmpty(t, r.Header.Get("x-lt-signature"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	}))
	list, err := client.QueryProductWarehouses(context.Background(), goodsCredential())
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, 2, list[1].Type)
	for _, invalid := range []string{`{"code":"0","info":{"list":null}}`, `{"code":"0","info":{"list":[{"warehouseCode":"wh-us","warehouseName":"US","warehouseType":3,"saleCountryList":["US"]}]}}`, `{"code":"0","info":{"list":[{"warehouseCode":"wh-us","warehouseName":"US","warehouseType":1,"saleCountryList":null}]}}`} {
		response = invalid
		_, err = client.QueryProductWarehouses(context.Background(), goodsCredential())
		require.ErrorIs(t, err, ErrGoodsUnavailable)
	}
	response = `{"code":"0","info":{"list":[]}}`
	list, err = client.QueryProductWarehouses(context.Background(), goodsCredential())
	require.NoError(t, err)
	require.NotNil(t, list)
	require.Empty(t, list)
}
