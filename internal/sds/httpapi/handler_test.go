package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	sdsclient "task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
)

type stubTemplateService struct {
	listCalls []sdstemplate.ListParams
	listFn    func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error)
	detailFn  func(productID string) (*sdstemplate.ProductDetail, error)
}

func (s *stubTemplateService) ListProducts(_ context.Context, params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
	s.listCalls = append(s.listCalls, params)
	if s.listFn != nil {
		return s.listFn(params)
	}
	return &sdstemplate.ListResponse{}, nil
}

func (s *stubTemplateService) GetProduct(_ context.Context, productID string) (*sdstemplate.ProductDetail, error) {
	if s.detailFn != nil {
		return s.detailFn(productID)
	}
	return &sdstemplate.ProductDetail{ProductSummary: sdstemplate.ProductSummary{ID: 99, Name: productID}}, nil
}

func TestCatalogListProductsAppliesLocalFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubTemplateService{
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
	router := newCatalogTestRouter(t, NewCatalogHandler(stub))

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

func TestCatalogCategoriesDeriveLeafCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubTemplateService{
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
	router := newCatalogTestRouter(t, NewCatalogHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/categories?shipmentArea=US", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var payload []CategorySummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, []CategorySummary{{ID: 20, Name: "Pillow", Count: 2}}, payload)
}

func TestCatalogUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newCatalogTestRouter(t, NewCatalogHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCatalogAuthRequiredReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubTemplateService{
		listFn: func(params sdstemplate.ListParams) (*sdstemplate.ListResponse, error) {
			return nil, &sdsclient.AuthRequiredError{Op: "GET /products", StatusCode: 400, Message: "用户未登录"}
		},
	}
	router := newCatalogTestRouter(t, NewCatalogHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "sds_auth_required", payload["error"])
	require.Contains(t, payload["message"], "SDS 登录状态已失效")
}

func newCatalogTestRouter(t *testing.T, handler HTTPRouteHandler) *gin.Engine {
	t.Helper()

	router := gin.New()
	mountRoutes(router, AppendRouteDescriptors(nil, handler))
	return router
}

func mountRoutes(router *gin.Engine, routes []httproute.Descriptor) {
	for _, route := range routes {
		router.Handle(route.Method, route.Path, route.Handler)
	}
}
