package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func useListingKitZitadelTestConfig(t *testing.T, cfg *listingKitZitadelRuntimeConfig) {
	t.Helper()
	t.Cleanup(SetListingKitZitadelAuthConfigForTesting(cfg))
}

func TestListingKitZitadelAuthRejectsMissingBearerToken(t *testing.T) {
	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: "https://issuer.example",
			ClientID:  "listingkit-client",
		},
	})

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
	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{Required: true},
	})

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
				"urn:zitadel:iam:org:project:roles": map[string]any{
					"listingkit_admin": map[string]any{"displayName": "ListingKit Admin"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

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
					"roles":     c.GetHeader("X-User-Roles"),
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
	if body["tenant_id"] != "org-286" || body["user_id"] != "user-42" || body["user_type"] != "zitadel" || body["roles"] != "listingkit_admin" {
		t.Fatalf("identity headers = %#v", body)
	}
}

func TestListingKitZitadelAuthRejectsUnauthorizedIdentity(t *testing.T) {
	zitadel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": r.Host + "/oauth/v2/authorize",
				"token_endpoint":         r.Host + "/oauth/v2/token",
				"introspection_endpoint": zitadelURL(r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "user-42",
				"username":                              "2-guest",
				"urn:zitadel:iam:user:resourceowner:id": "org-286",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
		AuthzConfig: zitadelAuthorizationConfig{
			Required:         true,
			AllowedUsernames: map[string]struct{}{"1-admin": {}},
		},
	})

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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusForbidden, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "zitadel_access_denied") {
		t.Fatalf("body = %s, want zitadel_access_denied", resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsConfiguredUsername(t *testing.T) {
	zitadel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": r.Host + "/oauth/v2/authorize",
				"token_endpoint":         r.Host + "/oauth/v2/token",
				"introspection_endpoint": zitadelURL(r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "user-1",
				"username":                              "1-admin",
				"urn:zitadel:iam:user:resourceowner:id": "org-286",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
		AuthzConfig: zitadelAuthorizationConfig{
			Required:         true,
			AllowedUsernames: map[string]struct{}{"1-admin": {}},
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/tasks",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"user_id": c.GetHeader("X-User-ID")})
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
}

func TestListingKitZitadelAuthRejectsAuthenticatedUserForOperationalAdminRoutesWithoutRole(t *testing.T) {
	zitadel := newZitadelRoleServer(t)
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/admin/stores",
			Module: "listing-kit-admin",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/stores", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusForbidden, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "listingkit_permission_denied") {
		t.Fatalf("body = %s, want listingkit_permission_denied", resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsOperatorForOperationalAdminRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t, "listingkit_operator")
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/admin/stores",
			Module: "listing-kit-admin",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/stores", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsAuthenticatedUserForSDSRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t)
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/sds/products",
			Module: "sds",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/products", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsAuthenticatedUserForTaskRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t)
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsAuthenticatedUserForAISettingsRoute(t *testing.T) {
	zitadel := newZitadelRoleServer(t)
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/settings/ai",
			Module: "listing-kit",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings/ai", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsAuthenticatedUserForSDSLoginRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t, "listingkit_operator")
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodPost,
			Path:   "/api/v1/sds-login/manual-login",
			Module: "sds-login",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sds-login/manual-login", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsAuthenticatedUserForRuleAdminRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t, "listingkit_operator")
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/admin/filter-rules",
			Module: "listing-kit-admin",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/admin/filter-rules", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthAllowsListingKitAdminForPlatformRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t, "listingkit_admin")
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/platform/subscriptions",
			Module: "listing-kit-platform-admin",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/platform/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func TestListingKitZitadelAuthRejectsOperatorForPlatformRoutes(t *testing.T) {
	zitadel := newZitadelRoleServer(t, "listingkit_operator")
	defer zitadel.Close()

	useListingKitZitadelTestConfig(t, &listingKitZitadelRuntimeConfig{
		AuthConfig: zitadelAuthConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "listingkit-client",
		},
	})

	router := gin.New()
	mountRoutes(router, []routeDescriptor{
		{
			Method: http.MethodGet,
			Path:   "/api/v1/listing-kits/platform/subscriptions",
			Module: "listing-kit-platform-admin",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/platform/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer access-token-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusForbidden, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "listingkit_permission_denied") {
		t.Fatalf("body = %s, want listingkit_permission_denied", resp.Body.String())
	}
}

func TestParseZitadelRoles(t *testing.T) {
	roles := parseZitadelRoles([]byte(`{
		"urn:zitadel:iam:org:project:roles": {"listingkit_admin": {}, "platform_admin": {}},
		"roles": ["platform_admin", "billing_admin"],
		"role": "admin, listingkit_admin"
	}`))

	want := []string{"listingkit_admin", "platform_admin", "billing_admin", "admin"}
	got := map[string]bool{}
	for _, role := range roles {
		got[role] = true
	}
	for _, role := range want {
		if !got[role] {
			t.Fatalf("roles = %#v, want role %q", roles, role)
		}
	}
}

func zitadelURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func newZitadelRoleServer(t *testing.T, roles ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": r.Host + "/oauth/v2/authorize",
				"token_endpoint":         r.Host + "/oauth/v2/token",
				"introspection_endpoint": zitadelURL(r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			roleMap := map[string]any{}
			for _, role := range roles {
				roleMap[role] = map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "user-42",
				"urn:zitadel:iam:user:resourceowner:id": "org-286",
				"urn:zitadel:iam:org:project:roles":     roleMap,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}
