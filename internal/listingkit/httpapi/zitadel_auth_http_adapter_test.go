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

func TestWrapZitadelAuthMiddlewareLetsCorsPreflightReachInnerHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Fatalf("method = %s, want OPTIONS", r.Method)
		}
		w.Header().Set("Access-Control-Allow-Origin", "https://app.example")
		w.WriteHeader(http.StatusNoContent)
	})
	middleware := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/crawl", nil)
	request.Header.Set("Origin", "https://app.example")
	WrapZitadelAuthMiddleware(next, middleware).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("CORS header = %q, want inner handler header", response.Header().Get("Access-Control-Allow-Origin"))
	}
}
