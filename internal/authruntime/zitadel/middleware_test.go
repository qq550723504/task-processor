package zitadel

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

func TestMiddlewareRejectsMissingConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := performRequest(t, NewMiddleware(Config{}, AuthorizationConfig{}), http.MethodGet, "/", "", nil, nil)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"error":"zitadel_auth_not_configured","message":"ZITADEL authentication is not configured"}`, rec.Body.String())
}

func TestMiddlewareRejectsMissingBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := performRequest(t, NewMiddleware(Config{
		IssuerURL: "https://issuer.example",
		ClientID:  "client-id",
	}, AuthorizationConfig{}), http.MethodGet, "/", "", nil, nil)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":"zitadel_token_missing","message":"Missing ZITADEL bearer token"}`, rec.Body.String())
}

func TestMiddlewareCachesDiscoveryDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var discoveryHits atomic.Int32
	var introspectionHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			discoveryHits.Add(1)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			introspectionHits.Add(1)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"active":                                true,
				"sub":                                   "user-42",
				"urn:zitadel:iam:user:resourceowner:id": "tenant-9",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	middleware := NewMiddleware(Config{
		IssuerURL: server.URL,
		ClientID:  "client-id",
	}, AuthorizationConfig{})

	for range 2 {
		rec := performRequest(t, middleware, http.MethodGet, "/", "access-token", nil, nil)
		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	}

	require.Equal(t, int32(1), discoveryHits.Load())
	require.Equal(t, int32(2), introspectionHits.Load())
}

func TestMiddlewareRejectsInvalidDiscoveryResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		handler    http.Handler
		httpClient *http.Client
	}{
		{
			name: "non-2xx",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", http.StatusBadGateway)
			}),
		},
		{
			name: "malformed",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "{")
			}),
		},
		{
			name: "transport",
			httpClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial failed")
				}),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				IssuerURL:  "https://issuer.example",
				ClientID:   "client-id",
				HTTPClient: tt.httpClient,
			}
			if tt.handler != nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				cfg.IssuerURL = server.URL
			}

			rec := performRequest(t, NewMiddleware(cfg, AuthorizationConfig{}), http.MethodGet, "/", "access-token", nil, nil)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.Contains(t, rec.Body.String(), "zitadel_token_invalid")
		})
	}
}

func TestMiddlewareProjectsVerifiedIdentityAndDeduplicatedRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			require.Equal(t, "access-token", r.FormValue("token"))
			requireBasicAuth(t, r, "client-id", "secret-value")
			writeJSON(t, w, http.StatusOK, map[string]any{
				"active":                                true,
				"sub":                                   "user-42",
				"username":                              "alice",
				"user_id":                               "legacy-user",
				"urn:zitadel:iam:user:resourceowner:id": "tenant-9",
				"roles":                                 []string{"operator"},
				"role":                                  "operator,admin",
				"urn:zitadel:iam:org:project:roles": map[string]any{
					"admin": map[string]any{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	headers := map[string]string{
		"X-Tenant-ID":     "forged-tenant",
		"tenant-id":       "forged-tenant",
		"X-Tenant":        "forged-tenant",
		"X-User-ID":       "forged-user",
		"X-User-Type":     "forged",
		"X-User-Roles":    "forged-role",
		"X-Zitadel-Roles": "forged-role",
		"X-User":          "forged-user",
	}

	rec := performRequest(t, NewMiddleware(Config{
		IssuerURL:    server.URL,
		ClientID:     "client-id",
		ClientSecret: "secret-value",
	}, AuthorizationConfig{}), http.MethodGet, "/", "access-token", headers, func(c *gin.Context) {
		identity, ok := authidentity.AuthenticatedIdentityFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, authidentity.AuthenticatedIdentity{
			TenantID: "tenant-9",
			UserID:   "user-42",
			Roles:    []string{"operator", "admin"},
		}, identity)

		c.JSON(http.StatusOK, gin.H{
			"tenant_id":     c.GetHeader("X-Tenant-ID"),
			"tenant_alias":  c.GetHeader("tenant-id"),
			"tenant_legacy": c.GetHeader("X-Tenant"),
			"user_id":       c.GetHeader("X-User-ID"),
			"user_type":     c.GetHeader("X-User-Type"),
			"user_legacy":   c.GetHeader("X-User"),
			"user_roles":    c.GetHeader("X-User-Roles"),
			"zitadel_roles": c.GetHeader("X-Zitadel-Roles"),
		})
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "tenant-9", body["tenant_id"])
	require.Equal(t, "tenant-9", body["tenant_alias"])
	require.Equal(t, "", body["tenant_legacy"])
	require.Equal(t, "user-42", body["user_id"])
	require.Equal(t, "zitadel", body["user_type"])
	require.Equal(t, "", body["user_legacy"])
	require.Equal(t, "operator,admin", body["user_roles"])
	require.Equal(t, "", body["zitadel_roles"])
}

func TestMiddlewareRejectsInvalidIntrospectionResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		handler    http.HandlerFunc
		httpClient *http.Client
	}{
		{
			name: "inactive",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, http.StatusOK, map[string]any{
						"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
					})
				case "/oauth/v2/introspect":
					writeJSON(t, w, http.StatusOK, map[string]any{"active": false})
				default:
					http.NotFound(w, r)
				}
			},
		},
		{
			name: "malformed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, http.StatusOK, map[string]any{
						"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
					})
				case "/oauth/v2/introspect":
					_, _ = io.WriteString(w, "{")
				default:
					http.NotFound(w, r)
				}
			},
		},
		{
			name: "non-2xx",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, http.StatusOK, map[string]any{
						"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
					})
				case "/oauth/v2/introspect":
					writeJSON(t, w, http.StatusBadGateway, map[string]any{"active": true})
				default:
					http.NotFound(w, r)
				}
			},
		},
		{
			name: "transport",
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]any{
					"introspection_endpoint": "https://issuer.example/oauth/v2/introspect",
				})
			},
			httpClient: &http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if strings.Contains(r.URL.Path, ".well-known") {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(`{"introspection_endpoint":"https://issuer.example/oauth/v2/introspect"}`)),
							Header:     make(http.Header),
							Request:    r,
						}, nil
					}
					return nil, errors.New("introspection transport failed")
				}),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				IssuerURL:  "https://issuer.example",
				ClientID:   "client-id",
				HTTPClient: tt.httpClient,
			}
			if tt.handler != nil && tt.httpClient == nil {
				server := httptest.NewServer(tt.handler)
				defer server.Close()
				cfg.IssuerURL = server.URL
			}

			rec := performRequest(t, NewMiddleware(cfg, AuthorizationConfig{}), http.MethodGet, "/", "access-token", nil, nil)
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.Contains(t, rec.Body.String(), "zitadel_token_invalid")
		})
	}
}

func TestMiddlewareRejectsMissingTenantAndSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name          string
		payload       map[string]any
		expectedCode  int
		expectedError string
	}{
		{
			name: "missing tenant",
			payload: map[string]any{
				"active": true,
				"sub":    "user-42",
			},
			expectedCode:  http.StatusForbidden,
			expectedError: "zitadel_tenant_missing",
		},
		{
			name: "missing subject",
			payload: map[string]any{
				"active":                                true,
				"urn:zitadel:iam:user:resourceowner:id": "tenant-9",
			},
			expectedCode:  http.StatusForbidden,
			expectedError: "zitadel_user_missing",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, http.StatusOK, map[string]any{
						"introspection_endpoint": serverURL(t, r) + "/oauth/v2/introspect",
					})
				case "/oauth/v2/introspect":
					writeJSON(t, w, http.StatusOK, tt.payload)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			rec := performRequest(t, NewMiddleware(Config{
				IssuerURL: server.URL,
				ClientID:  "client-id",
			}, AuthorizationConfig{}), http.MethodGet, "/", "access-token", nil, nil)

			require.Equal(t, tt.expectedCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.expectedError)
		})
	}
}

func TestMiddlewareAuthorizesWithFailClosedGlobalAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		server := newAuthServer(t, map[string]any{
			"active":                                true,
			"sub":                                   "user-42",
			"urn:zitadel:iam:user:resourceowner:id": "tenant-9",
			"roles":                                 []string{"operator"},
		})
		defer server.Close()

		rec := performRequest(t, NewMiddleware(Config{
			IssuerURL: server.URL,
			ClientID:  "client-id",
		}, AuthorizationConfig{
			Required:         true,
			AllowedTenantIDs: map[string]struct{}{"tenant-9": {}},
		}), http.MethodGet, "/", "access-token", nil, nil)

		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("fail closed without canonical allowlist", func(t *testing.T) {
		server := newAuthServer(t, map[string]any{
			"active":                                true,
			"sub":                                   "legacy-subject-value",
			"username":                              "display-name",
			"urn:zitadel:iam:user:resourceowner:id": "tenant-9",
		})
		defer server.Close()

		rec := performRequest(t, NewMiddleware(Config{
			IssuerURL: server.URL,
			ClientID:  "client-id",
		}, AuthorizationConfig{
			Required:                          true,
			LegacyUsernameAllowlistConfigured: true,
		}), http.MethodGet, "/", "access-token", nil, nil)

		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), "zitadel_access_denied")
	})
}

func TestParseRolesPreservesOrderAndDeduplicates(t *testing.T) {
	roles := ParseRoles([]byte(`{"roles":["operator"],"role":"operator,admin","urn:zitadel:iam:org:project:roles":{"admin":{}}}`))
	require.Equal(t, []string{"operator", "admin"}, roles)
}

func TestStringSliceToSetTrimsAndReturnsNilWhenEmpty(t *testing.T) {
	require.Nil(t, StringSliceToSet(nil))
	require.Nil(t, StringSliceToSet([]string{" ", "\t"}))
	require.Equal(t, map[string]struct{}{"a": {}, "b": {}}, StringSliceToSet([]string{" a ", "b", "a"}))
}

func TestNewMiddlewareNormalizesConfigAndUsesDefaultTimeout(t *testing.T) {
	impl := newMiddleware(Config{
		IssuerURL: "https://issuer.example/",
		ClientID:  " client-id ",
	}, AuthorizationConfig{})

	require.Equal(t, "https://issuer.example", impl.cfg.IssuerURL)
	require.Equal(t, "client-id", impl.cfg.ClientID)
	require.NotNil(t, impl.cfg.HTTPClient)
	require.Equal(t, 5*time.Second, impl.cfg.HTTPClient.Timeout)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func performRequest(t *testing.T, middleware gin.HandlerFunc, method string, path string, token string, headers map[string]string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	if handler == nil {
		handler = func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		}
	}

	router := gin.New()
	router.Use(middleware)
	router.Handle(method, path, handler)

	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func newAuthServer(t *testing.T, payload map[string]any) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"introspection_endpoint": server.URL + "/oauth/v2/introspect",
			})
		case "/oauth/v2/introspect":
			writeJSON(t, w, http.StatusOK, payload)
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func writeJSON(t *testing.T, w http.ResponseWriter, statusCode int, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func serverURL(t *testing.T, r *http.Request) string {
	t.Helper()

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func requireBasicAuth(t *testing.T, r *http.Request, clientID string, clientSecret string) {
	t.Helper()

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
	require.Equal(t, expected, r.Header.Get("Authorization"))
}
