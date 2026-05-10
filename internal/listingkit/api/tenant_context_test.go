package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestContextUsesYudaoGatewayHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("tenant-id", "286")
	req.Header.Set("login-user", url.QueryEscape(`{"id":42,"tenantId":286,"userType":2}`))
	c.Request = req

	if got := requestTenantID(c); got != "286" {
		t.Fatalf("tenant id = %q, want 286", got)
	}
	if got := requestUserID(c); got != "42" {
		t.Fatalf("user id = %q, want 42", got)
	}
}
