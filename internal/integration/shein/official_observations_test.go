package shein

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	o "task-processor/internal/marketplace/shein/observations"
	"testing"
	"time"
)

func observationResponse(raw string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(raw))}
}
func TestObservedProductsUsePublishedListAndPreserveUnknownInventory(t *testing.T) {
	c := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/open-api/goods/searchProduct", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		require.NotEmpty(t, r.Header.Get("x-lt-signature"))
		b, _ := io.ReadAll(r.Body)
		require.JSONEq(t, `{"pageNum":1,"pageSize":10,"languageList":["en"]}`, string(b))
		return observationResponse(`{"code":"0","info":{"meta":{"count":1},"data":[{"spuName":"spu-a","skcList":[{"skcName":"skc-a","skcTitle":[{"language":"en","title":"Product"}],"skcSiteShelfStatusList":[{"subSite":"shein-us","status":1}],"skuList":[{"skuCode":"sku-a","supplierSku":"mine","priceList":[{"site":"shein-us","currency":"USD","basePrice":12.5,"specialPrice":0}],"costList":[{"currency":"CNY","cost":"5.60"}],"inventoryList":[]}]}]}]},"secretFuture":"do-not-copy"}`), nil
	}))
	p, e := c.QueryObservedProducts(context.Background(), goodsCredential(), 1)
	require.NoError(t, e)
	require.Equal(t, 1, p.Total)
	require.Len(t, p.Items, 1)
	sku := p.Items[0].SKCs[0].SKUs[0]
	require.Empty(t, sku.Inventory)
	require.Equal(t, "12.5", sku.Prices[0].Value)
	require.Empty(t, sku.Prices[0].Special)
	require.Equal(t, "5.60", sku.Costs[0].Value)
	b, _ := json.Marshal(p)
	require.NotContains(t, string(b), "secretFuture")
}
func TestObservedOrderRequestUsesAllWarehousesAndResponseTimesAreUTC8(t *testing.T) {
	end := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	c := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/open-api/order/order-list", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		require.JSONEq(t, `{"queryType":1,"queryOrderType":4,"startTime":"2026-10-08 20:00:00","endTime":"2026-10-09 20:00:00","page":1,"pageSize":30}`, string(b))
		return observationResponse(`{"code":0,"info":{"count":1,"orderList":[{"orderNo":"order-a","orderStatus":"2","orderCreateTime":"2026-10-09 09:00:00","orderUpdateTime":"2026-10-09 10:00:00"}]}}`), nil
	}))
	p, e := c.QueryObservedOrders(context.Background(), goodsCredential(), o.Window{Start: end.Add(-24 * time.Hour), End: end}, 1)
	require.NoError(t, e)
	require.Equal(t, time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC), p.Items[0].CreatedAt)
	_, e = c.QueryObservedOrders(context.Background(), goodsCredential(), o.Window{Start: end.Add(-49 * time.Hour), End: end}, 1)
	require.ErrorIs(t, e, o.ErrInvalid)
}
func TestObservedOrderDetailsCorrelateMembershipAndDiscardPrivateFields(t *testing.T) {
	raw := `{"code":"0","info":[{"orderNo":"order-a","salesSite":"shein-us","orderStatus":2,"orderCurrency":"USD","productTotalPrice":"19.90","orderTime":"2026-10-09T10:00:00.000+0800","needDeliveryTime":"2026-10-11T10:00:00.000+0800","receiveMsg":{"city":"private-city"},"paymentInfo":{"payNo":"private-payment"},"orderGoodsInfoList":[{"goodsId":9007199254740993,"goodsTitle":"Product","skuCode":"sku","sellerSku":"mine","customizationInfo":{"texts":["private-text"]}}],"packageWaybillList":[{"packageNo":"package-a"}]}]}`
	c := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) { return observationResponse(raw), nil }))
	items, e := c.QueryObservedOrderDetails(context.Background(), goodsCredential(), []string{"order-a"})
	require.NoError(t, e)
	require.Equal(t, "9007199254740993", items[0].Items[0].ID)
	require.Equal(t, "19.90", items[0].Amount.Value)
	require.Equal(t, "package-a", items[0].Packages[0].ID)
	b, _ := json.Marshal(items)
	for _, x := range []string{"private-city", "private-payment", "private-text", "receiveMsg", "paymentInfo"} {
		require.NotContains(t, string(b), x)
	}
	_, e = c.QueryObservedOrderDetails(context.Background(), goodsCredential(), []string{"foreign-order"})
	require.ErrorIs(t, e, o.ErrUnavailable)
	raw = `{"code":"0","info":[{"orderNo":"order-a"},{"orderNo":"order-a"}]}`
	_, e = c.QueryObservedOrderDetails(context.Background(), goodsCredential(), []string{"order-a"})
	require.ErrorIs(t, e, o.ErrUnavailable)
}
func TestObservedLogisticsAllowsPackageWithoutWaybillAndRejectsDuplicateJSON(t *testing.T) {
	raw := `{"code":"0","info":{"trackInfo":[{"carrier":"Carrier","waybillNo":"waybill-a","tracking":[{"description":"Collected","nodeCode":"collected","updateTimeMillis":1700000000000}]}]}}`
	c := testClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/open-api/gsp/logistics-track", r.URL.Path)
		require.Equal(t, "order-a", r.URL.Query().Get("orderNo"))
		require.Equal(t, "package-a", r.URL.Query().Get("packageNo"))
		require.False(t, r.URL.Query().Has("waybillNo"))
		return observationResponse(raw), nil
	}))
	tracks, e := c.QueryObservedTrack(context.Background(), goodsCredential(), "order-a", "package-a")
	require.NoError(t, e)
	require.Equal(t, "Collected", tracks[0].Nodes[0].Description)
	raw = `{"code":"0","code":"0","info":{"trackInfo":[]}}`
	_, e = c.QueryObservedTrack(context.Background(), goodsCredential(), "order-a", "package-a")
	require.ErrorIs(t, e, o.ErrUnavailable)
}
func TestObservedLogisticsSuccessfulMissingTrackIsEmptyNotFailure(t *testing.T) {
	for _, raw := range []string{`{"code":0}`, `{"code":"0","info":null}`, `{"code":0,"info":{}}`, `{"code":0,"info":{"trackInfo":null}}`} {
		t.Run(raw, func(t *testing.T) {
			c := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) { return observationResponse(raw), nil }))
			tracks, e := c.QueryObservedTrack(context.Background(), goodsCredential(), "order-a", "package-a")
			require.NoError(t, e)
			require.Equal(t, []o.Track{}, tracks)
		})
	}
	c := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) { return observationResponse(`{"code":1}`), nil }))
	_, e := c.QueryObservedTrack(context.Background(), goodsCredential(), "order-a", "package-a")
	require.ErrorIs(t, e, o.ErrUnavailable)
}
func TestObservedProductsPreserveUnrecognizedPlatformStatus(t *testing.T) {
	c := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return observationResponse(`{"code":0,"info":{"meta":{"count":1},"data":[{"spuName":"spu-a","skcList":[{"skcName":"skc-a","skcSiteShelfStatusList":[{"subSite":"shein-us","status":42}],"skuList":[]}]}]}}`), nil
	}))
	p, e := c.QueryObservedProducts(context.Background(), goodsCredential(), 1)
	require.NoError(t, e)
	require.Equal(t, 42, *p.Items[0].SKCs[0].SiteStatus)
}
