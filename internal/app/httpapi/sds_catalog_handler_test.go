package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	sdsclient "task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
)

type stubSDSCatalogTemplateService struct {
	listCalls []sdstemplate.ListParams
	listFn    func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error)
	detailFn  func(productID string) (*sdstemplate.ProductDetail, error)
}

func (s *stubSDSCatalogTemplateService) ListProducts(_ context.Context, params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
	s.listCalls = append(s.listCalls, params)
	if s.listFn != nil {
		return s.listFn(params)
	}
	return &sdstemplate.ListResponse{}, nil
}

func (s *stubSDSCatalogTemplateService) GetProduct(_ context.Context, productID string) (*sdstemplate.ProductDetail, error) {
	if s.detailFn != nil {
		return s.detailFn(productID)
	}
	return &sdstemplate.ProductDetail{ProductSummary: sdstemplate.ProductSummary{ID: 99, Name: productID}}, nil
}

func TestSDSCatalogListProductsAppliesLocalFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubSDSCatalogTemplateService{
		listFn: func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
			return &sdstemplate.ListResponse{
				Page:       params.Page,
				Size:       params.Size,
				TotalCount: 3,
				Items: []sdstemplate.ProductSummary{
					{ID: 1, Name: "light", Weight: 120, ProductionCycle: 12},
					{ID: 2, Name: "medium", Weight: 300, ProductionCycle: 48},
					{ID: 3, Name: "heavy", Weight: 1200, ProductionCycle: 96},
				},
			}, nil
		},
	}
	router := newSDSCatalogTestRouter(t, newSDSCatalogHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products?weightBand=200-500&cycleBand=24-72&page=1&size=12", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var payload sdstemplate.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, 1, payload.TotalCount)
	require.Len(t, payload.Items, 1)
	require.Equal(t, int64(2), payload.Items[0].ID)
	require.Equal(t, 100, stub.listCalls[0].Size)
}

func TestSDSCatalogCategoriesDeriveLeafCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubSDSCatalogTemplateService{
		listFn: func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
			return &sdstemplate.ListResponse{
				TotalCount: 2,
				Items: []sdstemplate.ProductSummary{
					{ID: 1, Categories: []sdstemplate.Category{{ID: 10, Name: "Root"}, {ID: 20, Name: "Pillow"}}},
					{ID: 2, Categories: []sdstemplate.Category{{ID: 10, Name: "Root"}, {ID: 20, Name: "Pillow"}}},
				},
			}, nil
		},
	}
	router := newSDSCatalogTestRouter(t, newSDSCatalogHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/categories?shipmentArea=US", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var payload []sdsCategorySummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, []sdsCategorySummary{{ID: 20, Name: "Pillow", Count: 2}}, payload)
}

func TestSDSCatalogUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newSDSCatalogTestRouter(t, newSDSCatalogHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSDSCatalogAuthRequiredReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubSDSCatalogTemplateService{
		listFn: func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
			return nil, &sdsclient.AuthRequiredError{Op: "GET /products", StatusCode: 400, Message: "用户未登录"}
		},
	}
	router := newSDSCatalogTestRouter(t, newSDSCatalogHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "sds_auth_required", payload["error"])
	require.Contains(t, payload["message"], "SDS 登录状态已失效")
}

func newSDSCatalogTestRouter(t *testing.T, handler sdsCatalogRouteHandler) *gin.Engine {
	t.Helper()

	router := gin.New()
	RegisterRoutes(router, nil, nil, nil, nil, nil, handler)
	return router
}
