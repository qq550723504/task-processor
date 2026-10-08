package supplychainapp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"task-processor/internal/listing/preparation"
	"testing"
)

func TestSupplyOptimizationRequiresConfiguredAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	for _, route := range SupplyRoutes(&Application{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
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
	for _, route := range SupplyRoutes(&Application{}, func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }) {
		router.Handle(route.Method, route.Path, route.Handler)
	}
	request := httptest.NewRequest(http.MethodPost, SupplyBasePath+"/operations/11111111-1111-4111-8111-111111111111/ensure", strings.NewReader(`{"unexpected":true}`))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
}
