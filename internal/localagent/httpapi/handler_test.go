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

func TestSubmitResultReconstructsEnvelopeAndDoesNotAcceptSourceAccount(t *testing.T) {
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
	var done localagent.Job
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &done))
	require.Equal(t, localagent.JobSucceeded, done.State)
	require.Equal(t, "crawler:1688:1052008074197", done.Envelope.Identity.SourceKey())
	require.Zero(t, done.Envelope.Identity.StoreID)
	require.Equal(t, "1052008074197", done.Envelope.RawReference.ReferenceID)
}

func TestHandlersRequireVerifiedIdentity(t *testing.T) {
	handler := NewHandler(localagent.NewService(nil))
	r := gin.New()
	r.POST("/api/v1/local-agent/1688-jobs", handler.Create)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/local-agent/1688-jobs", strings.NewReader(`{"url":"https://detail.1688.com/offer/1052008074197.html"}`)))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}
