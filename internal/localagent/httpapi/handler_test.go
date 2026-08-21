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

	"task-processor/internal/listingkit"
	"task-processor/internal/localagent"
)

func TestCreateJobUsesVerifiedTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := localagent.NewService(func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) })
	handler := NewHandler(service)
	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs", strings.NewReader(`{"url":"https://detail.1688.com/offer/1052008074197.html","tenant_id":"forged"}`))
	req = req.WithContext(listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{TenantID: "tenant-real", UserID: "user-1"}))
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)

	require.Equal(t, http.StatusCreated, response.Code)
	require.NotContains(t, response.Body.String(), "execution_token")
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "tenant-real", body["tenant_id"])
}

func TestSubmitResultAcknowledgesTerminalJobAndDoesNotAcceptSourceAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clock := func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	service := localagent.NewService(clock)
	handler := NewHandler(service)
	actorCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})

	create := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs", strings.NewReader(`{"url":"https://detail.1688.com/offer/1052008074197.html"}`)).WithContext(actorCtx)
	createRouter := gin.New()
	createRouter.POST("/api/v1/local-agent/1688-jobs", handler.Create)
	createRouter.ServeHTTP(create, createReq)
	var created jobResponse
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &created))

	claim := httptest.NewRecorder()
	claimRouter := gin.New()
	claimRouter.POST("/api/v1/local-agent/1688-jobs/claim", handler.Claim)
	claimRouter.ServeHTTP(claim, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs/claim", nil).WithContext(actorCtx))
	require.Equal(t, http.StatusOK, claim.Code)
	var claimed claimResponse
	require.NoError(t, json.Unmarshal(claim.Body.Bytes(), &claimed))

	resultRouter := gin.New()
	resultRouter.POST("/api/v1/local-agent/1688-jobs/:job_id/result", handler.SubmitResult)
	bodyBytes, err := json.Marshal(map[string]any{"execution_token": claimed.ExecutionToken, "source_account_id": 42, "product_snapshot": map[string]string{"id": "1052008074197", "title": "shirt", "url": "https://detail.1688.com/offer/1052008074197.html"}})
	require.NoError(t, err)
	result := httptest.NewRecorder()
	resultRouter.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs/"+created.JobID+"/result", strings.NewReader(string(bodyBytes))).WithContext(actorCtx))
	require.Equal(t, http.StatusOK, result.Code)
	var done terminalResponse
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &done))
	require.Equal(t, localagent.JobSucceeded, done.State)
	require.Equal(t, created.JobID, done.JobID)
	require.NotNil(t, done.EnvelopeSummary)
	require.Equal(t, "crawler:1688:1052008074197", done.EnvelopeSummary.SourceKey)
	require.NotContains(t, result.Body.String(), "source_account_id")
	require.NotContains(t, result.Body.String(), `"envelope":`)
}

func TestSubmitResultExposesInvalidSnapshotError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clock := func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	service := localagent.NewService(clock)
	handler := NewHandler(service)
	actorCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	job, err := service.Create(localagent.Actor{TenantID: "tenant-a", UserID: "user-a"}, "https://detail.1688.com/offer/1052008074197.html")
	require.NoError(t, err)
	claim, err := service.Claim(localagent.Actor{TenantID: "tenant-a", UserID: "user-a"})
	require.NoError(t, err)

	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs/:job_id/result", handler.SubmitResult)
	response := httptest.NewRecorder()
	body := `{"execution_token":"` + claim.ExecutionToken + `","product_snapshot":{"id":"999","title":"shirt","url":"https://detail.1688.com/offer/1052008074197.html"}}`
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs/"+job.ID+"/result", strings.NewReader(body)).WithContext(actorCtx))

	require.Equal(t, http.StatusBadRequest, response.Code)
	var responseBody map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &responseBody))
	require.Equal(t, "snapshot_invalid", responseBody["error"])
}

func TestProductSnapshotRequestMapsSnakeCaseFields(t *testing.T) {
	var req productSnapshotRequest
	err := json.Unmarshal([]byte(`{"id":"1052008074197","main_image":"https://img/main.jpg","min_price":12.5,"price_range_count":2,"supplier":{"company_name":"Acme","years_in_business":8},"pack_info":{"package_type":"box","package_images":["https://img/pack.jpg"]},"variants":[{"attributes":{"Color":"red"},"stock":7,"price":13.25}],"shipping":{"shipping_from":"Hangzhou","processing_time":"3 days"},"is_customized":true}`), &req)
	require.NoError(t, err)
	snapshot := req.toSnapshot()
	require.Equal(t, "https://img/main.jpg", snapshot.MainImage)
	require.Equal(t, 12.5, snapshot.MinPrice)
	require.Equal(t, 2, snapshot.PriceRangeCount)
	require.Equal(t, "Acme", snapshot.Supplier.CompanyName)
	require.Equal(t, 8, snapshot.Supplier.YearsInBusiness)
	require.Equal(t, "box", snapshot.PackInfo.PackageType)
	require.Equal(t, "red", snapshot.Variants[0].Attributes["Color"])
	require.Equal(t, "Hangzhou", snapshot.Shipping.ShippingFrom)
	require.True(t, snapshot.IsCustomized)
}

func TestCreateRejectsOversizedRequestBodyBeforeBinding(t *testing.T) {
	service := localagent.NewService(nil)
	handler := NewHandler(service)
	actorCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs", handler.Create)
	response := httptest.NewRecorder()
	body := `{"url":"https://detail.1688.com/offer/1052008074197.html","padding":"` + strings.Repeat("x", maxCreateBodyBytes) + `"}`
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs", strings.NewReader(body)).WithContext(actorCtx))
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestHandlersRequireVerifiedIdentity(t *testing.T) {
	handler := NewHandler(localagent.NewService(nil))
	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs", handler.Create)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs", strings.NewReader(`{"url":"https://detail.1688.com/offer/1052008074197.html"}`)))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestSubmitResultRejectsOversizedRequestBodyBeforeBinding(t *testing.T) {
	service := localagent.NewService(nil)
	handler := NewHandler(service)
	actorCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs/:job_id/result", handler.SubmitResult)
	response := httptest.NewRecorder()
	body := `{"execution_token":"token","product_snapshot":{"id":"1052008074197","url":"https://detail.1688.com/offer/1052008074197.html","title":"` + strings.Repeat("x", 2<<20) + `"}}`
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs/job-1/result", strings.NewReader(body)).WithContext(actorCtx))
	require.Equal(t, http.StatusBadRequest, response.Code)
	var responseBody map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &responseBody))
	require.Equal(t, "snapshot_too_large", responseBody["error"])
}
