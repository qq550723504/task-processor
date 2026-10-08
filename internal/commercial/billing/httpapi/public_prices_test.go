package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
	"task-processor/internal/ledger/orgresource"
)

type publicTestCatalog struct {
	billing.OfferCatalog
	items []billing.Offer
}

func (s publicTestCatalog) ListResourceOffers(context.Context) ([]billing.Offer, error) {
	return s.items, nil
}

type publicTestQuotes struct{ billing.QuoteEngine }
type publicTestOrders struct{ billing.OrderStore }
type publicTestReader struct{ billing.OrderReader }
type publicTestWallet struct{ billing.WalletPort }
type publicTestGrants struct {
	billing.PurchasedResourceGrantPort
}

func TestPublicCatalogOnlyExposesSellablePrices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := publicTestCatalog{items: []billing.Offer{{OfferID: "store-service-30d-v1", ProductKind: billing.ProductStoreRenewalPeriod, ResourceType: orgresource.ResourceStoreRenewalPeriod, Currency: "CNY", UnitPriceMinor: 16800, PricingVersion: "store-168-v1", MinQuantity: 1, MaxQuantity: 120, Status: billing.OfferActive}}}
	service, err := billing.NewService(catalog, publicTestQuotes{}, publicTestOrders{}, publicTestReader{}, publicTestWallet{}, publicTestGrants{})
	require.NoError(t, err)
	routes, err := Routes(NewHandler(service))
	require.NoError(t, err)
	engine := gin.New()
	found := false
	for _, r := range routes {
		if r.Path == "/api/v1/commercial/resource-offers" {
			found = true
			require.Equal(t, httproute.AuthPolicyPublic, r.AuthPolicy)
			require.Equal(t, httproute.OrganizationAccessPolicyNone, r.OrganizationAccessPolicy)
			require.Empty(t, r.Permission)
			require.Nil(t, r.OrganizationTargetResolver)
			require.True(t, r.RejectUnreadRequestBody)
			require.Equal(t, 5*time.Second, r.RequestTimeout)
			engine.Handle(r.Method, r.Path, r.Handler)
		} else if strings.HasPrefix(r.Path, "/api/v1/workbench/commercial/") {
			require.Equal(t, httproute.AuthPolicyCurrentIdentity, r.AuthPolicy, "private route must retain auth")
			require.NotEmpty(t, r.Permission)
		}
	}
	require.True(t, found, "homepage needs an anonymous price catalog, separate from private enterprise reads")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/commercial/resource-offers", nil))
	require.Equal(t, 200, response.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Len(t, body, 3)
	require.Equal(t, "retail-price-catalog-v1", body["schema_version"])
	require.Equal(t, float64(30), body["store_period_days"])
	items := body["items"].([]any)
	item := items[0].(map[string]any)
	require.Len(t, item, 8)
	require.Equal(t, "16800", item["unit_price_minor"])
	for _, target := range []struct{ url, body string }{{"/api/v1/commercial/resource-offers?organization_id=secret-org", ""}, {"/api/v1/commercial/resource-offers?", ""}, {"/api/v1/commercial/resource-offers", "private-body"}} {
		r := httptest.NewRecorder()
		engine.ServeHTTP(r, httptest.NewRequest(http.MethodGet, target.url, strings.NewReader(target.body)))
		require.Equal(t, 400, r.Code)
	}
}
