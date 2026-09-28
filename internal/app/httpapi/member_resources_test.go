package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	billing "task-processor/internal/commercial/billing"
	"task-processor/internal/httproute"
	resourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/organization/membership"
)

func TestMemberResourcesHTTPUsesNativePositionsAndAllowsDepartedFreeReclaim(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "resource.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, _ := db.DB()
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, resourceadapter.AutoMigrate(db))
	require.NoError(t, commercialstore.AutoMigrate(db))
	require.NoError(t, db.Table("saas_organization_resource_buckets").Create(map[string]any{"organization_id": "org-1", "resource_type": "store_renewal_period", "available": 4, "reserved": 0, "consumed": 0, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error)
	positions, err := resourceadapter.NewGormMemberAllocationRepository(db, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	directory := &pointLimitDirectory{member: membership.Member{ID: "member-1", UserID: "user-1", OrganizationID: "org-1", ProjectID: "project-1", State: "active", DisplayName: "Member One", Roles: []string{"listingkit_operator"}}}
	gate := memberResourceGate{base: memberPointLimitAuthorizer{authorizer: authz.DefaultListingKitAuthorizer(), directory: directory, projectID: "project-1"}}
	service, err := orgresource.NewMemberAllocationService(positions, gate)
	require.NoError(t, err)
	prices, err := commercialstore.New(db)
	require.NoError(t, err)
	require.NoError(t, prices.SaveOffer(context.Background(), billing.Offer{OfferID: "data-price", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: billing.CurrencyCNY, PricingVersion: "price-v1", UnitPriceMinor: 3, MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive}))
	require.NoError(t, db.Table("saas_organization_resource_buckets").Create(map[string]any{"organization_id": "org-1", "resource_type": "data_row", "available": 10, "reserved": 0, "consumed": 0, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error)
	module := memberResourcesModule{positions: positions, service: service, gate: gate, prices: prices}
	role := "listingkit_admin"
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authidentity.WithAuthenticatedIdentity(c.Request.Context(), authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", UserID: "admin", EffectiveMemberID: "admin-member", Roles: []string{role}, OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-1", AuthorizationID: "admin-member"}}}))
		c.Next()
	})
	for _, route := range module.routes() {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Requested-Organization-ID", "org-1")
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	path := memberResourcesBase + "/members/member-1/transfers"
	first := request("POST", path, `{"resourceType":"store_renewal_period","action":"allocate","quantity":"2","expectedVersion":"0"}`, "allocate-1")
	require.Equal(t, 200, first.Code, first.Body.String())
	replay := request("POST", path, `{"resourceType":"store_renewal_period","action":"allocate","quantity":"2","expectedVersion":"0"}`, "allocate-1")
	require.Equal(t, 200, replay.Code, replay.Body.String())
	changed := request("POST", path, `{"resourceType":"store_renewal_period","action":"allocate","quantity":"3","expectedVersion":"0"}`, "allocate-1")
	require.Equal(t, 409, changed.Code, changed.Body.String())
	quoteResponse := request("POST", memberResourcesBase+"/data-quotes", `{"offerId":"data-price","amountMinor":"10"}`, "")
	require.Equal(t, 200, quoteResponse.Code, quoteResponse.Body.String())
	var quote struct {
		QuoteID   string `json:"quoteId"`
		Quantity  string `json:"quantity"`
		Remainder string `json:"remainderMinor"`
	}
	require.NoError(t, json.Unmarshal(quoteResponse.Body.Bytes(), &quote))
	require.Equal(t, "3", quote.Quantity)
	require.Equal(t, "1", quote.Remainder)
	dataBody := `{"resourceType":"data_row","action":"allocate","quantity":"3","expectedVersion":"0","quoteId":"` + quote.QuoteID + `"}`
	data := request("POST", path, dataBody, "data-allocate")
	require.Equal(t, 200, data.Code, data.Body.String())
	require.NoError(t, db.Table("commercial_quotes").Where("quote_id=?", quote.QuoteID).Update("expires_at", time.Now().UTC().Add(-time.Second)).Error)
	dataReplay := request("POST", path, dataBody, "data-allocate")
	require.Equal(t, 200, dataReplay.Code, dataReplay.Body.String())
	require.Contains(t, dataReplay.Body.String(), `"replayed":true`)
	expired := request("POST", path, strings.Replace(dataBody, `"expectedVersion":"0"`, `"expectedVersion":"1"`, 1), "new-expired")
	require.Equal(t, 409, expired.Code, expired.Body.String())
	duplicate := request("POST", path, `{"resourceType":"store_renewal_period","action":"allocate","quantity":"1","quantity":"2","expectedVersion":"1"}`, "duplicate")
	require.Equal(t, 400, duplicate.Code, duplicate.Body.String())
	directory.member.State = "removed"
	read := request("GET", memberResourcesBase, "", "")
	require.Equal(t, 200, read.Code, read.Body.String())
	require.Contains(t, read.Body.String(), `"free":"2"`)
	require.Contains(t, read.Body.String(), `"state":"removed"`)
	denied := request("POST", path, `{"resourceType":"store_renewal_period","action":"allocate","quantity":"1","expectedVersion":"1"}`, "allocate-inactive")
	require.Equal(t, 403, denied.Code, denied.Body.String())
	reclaim := request("POST", path, `{"resourceType":"store_renewal_period","action":"reclaim","quantity":"1","expectedVersion":"1"}`, "reclaim-1")
	require.Equal(t, 200, reclaim.Code, reclaim.Body.String())
	require.Contains(t, reclaim.Body.String(), `"free":"1"`)
	role = "listingkit_viewer"
	blocked := request("POST", path, `{"resourceType":"store_renewal_period","action":"reclaim","quantity":"1","expectedVersion":"2"}`, "viewer-reclaim")
	require.Equal(t, http.StatusForbidden, blocked.Code, blocked.Body.String())
	position, err := positions.ReadPosition(context.Background(), "org-1", "member-1", orgresource.ResourceStoreRenewalPeriod)
	require.NoError(t, err)
	require.EqualValues(t, 1, position.Free)
}

func TestCurrentMemberResourceRoutesPreserveExactReadAndWriteBoundary(t *testing.T) {
	var routes []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	routes = append(routes, (memberResourcesModule{}).routes()...)
	validate := func(r []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{MemberResources: true})
	}
	require.NoError(t, validate(routes))
	for i, route := range routes {
		if route.Module != memberResourcesModuleName {
			continue
		}
		changed := append([]httproute.Descriptor(nil), routes...)
		changed[i].Permission = authz.PermissionWorkbenchOrganizationMemberRead
		if route.Method != "GET" {
			require.Error(t, validate(changed))
		}
		changed[i] = route
		changed[i].OrganizationTargetResolver = nil
		require.Error(t, validate(changed))
	}
}
