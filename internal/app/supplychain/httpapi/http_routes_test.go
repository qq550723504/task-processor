package supplychainhttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"testing"
)

func TestSupplyOptimizationRequiresConfiguredAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	for _, route := range SupplyRoutes(&supplyapp.Application{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	id := "11111111-1111-4111-8111-111111111111"
	request := httptest.NewRequest(http.MethodPost, SupplyBasePath+"/operations", strings.NewReader(`{"preparationId":"`+id+`","expectedRevision":1,"action":"optimize","storeId":"`+id+`","titleTemplateId":"title-review-v1"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", id)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusConflict, response.Code)
	require.Contains(t, response.Body.String(), "NOT_READY")
}

func TestSupplyRejectsMalformedQueryInsteadOfSilentlyDroppingIt(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.URL.RawQuery = "keyword=%ZZ"
	_, err := supplyQuery(request)
	require.ErrorIs(t, err, preparation.ErrInvalid)
}
func TestSupplyEnsureRejectsChunkedBody(t *testing.T) {
	router := gin.New()
	for _, route := range SupplyRoutes(&supplyapp.Application{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	request := httptest.NewRequest(http.MethodPost, SupplyBasePath+"/operations/11111111-1111-4111-8111-111111111111/ensure", strings.NewReader(`{"unexpected":true}`))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestOptimizationOptionsAcceptsBoundedCurrentPageQuery(t *testing.T) {
	called := false
	app := &supplyapp.Application{OptimizationOptions: func(_ context.Context, q collection.Query) (supplyapp.OptimizationOptions, error) {
		called = true
		require.Equal(t, 100, q.Limit)
		return supplyapp.OptimizationOptions{Titles: []supplyapp.TitleOptimizationChoice{}}, nil
	}}
	router := gin.New()
	for _, route := range SupplyRoutes(app, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, SupplyBasePath+"/optimization-options?limit=100", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.True(t, called)
	for _, query := range []string{"limit=101", "limit=1&limit=2", "keyword=anything", "stage=ready"} {
		called = false
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, SupplyBasePath+"/optimization-options?"+query, nil))
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.False(t, called)
	}
}
