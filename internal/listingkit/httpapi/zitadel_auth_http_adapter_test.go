package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"task-processor/internal/listingkit"
)

func TestWrapZitadelAuthMiddlewareForwardsVerifiedIdentity(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := listingkit.AuthenticatedIdentityFromContext(r.Context())
		if !ok || identity.TenantID != "101" {
			http.Error(w, "verified identity missing", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := func(c *gin.Context) {
		c.Request = c.Request.WithContext(listingkit.WithAuthenticatedIdentity(c.Request.Context(), listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-101"}))
		c.Next()
	}

	response := httptest.NewRecorder()
	WrapZitadelAuthMiddleware(next, middleware).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/crawl", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
