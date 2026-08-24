package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"task-processor/internal/listingsubscription"
)

func TestWriteStudioBatchActionErrorMapsLedgerQuotaToPaymentRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	limit := int64(2)
	err := &listingsubscription.UsageQuotaError{
		TenantID: "tenant-1", ModuleCode: listingsubscription.ModuleStudio,
		Metric: "product_image_jobs_succeeded", Limit: &limit, Quantity: 1,
	}
	writeStudioBatchActionError(c, "studio_batch_tasks_failed", errors.Join(errors.New("wrapped"), err))
	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPaymentRequired)
	}
	if recorder.Body.String() == "" || !containsQuotaExceeded(recorder.Body.String()) {
		t.Fatalf("body = %q, want quota_exceeded", recorder.Body.String())
	}
}

func containsQuotaExceeded(body string) bool {
	return len(body) >= len("quota_exceeded") &&
		stringContains(body, "quota_exceeded")
}

func stringContains(body, value string) bool {
	for i := 0; i+len(value) <= len(body); i++ {
		if body[i:i+len(value)] == value {
			return true
		}
	}
	return false
}
