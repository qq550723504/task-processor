package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListingKitZitadelAuthRejectsMissingBearerToken(t *testing.T) {
	t.Setenv("ZITADEL_ISSUER_URL", "https://issuer.example")
	t.Setenv("ZITADEL_CLIENT_ID", "listingkit-client")

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil))

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusUnauthorized, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "zitadel_token_missing") {
		t.Fatalf("body = %s, want zitadel_token_missing", resp.Body.String())
	}
}

func TestListingKitZitadelAuthReturnsUnavailableWhenRequiredButNotConfigured(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED", "1")

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil))

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusServiceUnavailable, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "zitadel_auth_not_configured") {
		t.Fatalf("body = %s, want zitadel_auth_not_configured", resp.Body.String())
	}
}

func TestListingKitZitadelAuthMapsVerifiedIdentityToHeaders(t *testing.T) {
	var introspectionToken string
	zitadel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": r.Host + "/oauth/v2/authorize",
				"token_endpoint":         r.Host + "/oauth/v2/token",
				"introspection_endpoint": zitadelURL(r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			introspectionToken = r.FormValue("token")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "user-42",
				"urn:zitadel:iam:user:resourceowner:id": "org-286",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()

	t.Setenv("ZITADEL_ISSUER_URL", zitadel.URL)
	t.Setenv("ZITADEL_CLIENT_ID", "listingkit-client")

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"tenant_id": c.GetHeader("X-Tenant-ID"),
					"user_id":   c.GetHeader("X-User-ID"),
					"user_type": c.GetHeader("X-User-Type"),
				})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if introspectionToken != "access-token-1" {
		t.Fatalf("introspection token = %q, want access-token-1", introspectionToken)
	}
	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["tenant_id"] != "org-286" || body["user_id"] != "user-42" || body["user_type"] != "zitadel" {
		t.Fatalf("identity headers = %#v", body)
	}
}

func zitadelURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
