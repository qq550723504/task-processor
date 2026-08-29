package zitadel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

func TestVerifierReturnsCanonicalIdentity(t *testing.T) {
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
			require.Equal(t, "user-token", r.FormValue("token"))
			require.Equal(t, "access_token", r.FormValue("token_type_hint"))
			requireBasicAuth(t, r, "api", "secret")
			writeJSON(t, w, http.StatusOK, map[string]any{
				"active":                                true,
				"sub":                                   " user-1 ",
				"urn:zitadel:iam:user:resourceowner:id": " org-1 ",
				"roles":                                 []string{"listingkit_operator"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	verifier := NewVerifier(Config{
		IssuerURL: server.URL, ClientID: "api", ClientSecret: "secret",
		HTTPClient: server.Client(),
	})

	for range 2 {
		got, err := verifier.Verify(context.Background(), "user-token")
		require.NoError(t, err)
		require.Equal(t, authidentity.AuthenticatedIdentity{
			TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"},
		}, got)
	}

	require.Equal(t, int32(1), discoveryHits.Load())
	require.Equal(t, int32(2), introspectionHits.Load())
}

func TestVerifierRejectsInactiveAndIncompleteIdentity(t *testing.T) {
	testCases := []struct {
		name         string
		payload      map[string]any
		expectedText string
	}{
		{
			name: "inactive token",
			payload: map[string]any{
				"active": false,
			},
			expectedText: "ZITADEL token introspection returned an inactive token",
		},
		{
			name: "missing resource owner",
			payload: map[string]any{
				"active": true,
				"sub":    "user-1",
			},
			expectedText: "ZITADEL resource owner is required",
		},
		{
			name: "missing subject",
			payload: map[string]any{
				"active":                                true,
				"urn:zitadel:iam:user:resourceowner:id": "org-1",
			},
			expectedText: "ZITADEL subject is required",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := newAuthServer(t, tt.payload)
			defer server.Close()

			verifier := NewVerifier(Config{
				IssuerURL:  server.URL,
				ClientID:   "api",
				HTTPClient: server.Client(),
			})

			_, err := verifier.Verify(context.Background(), "user-token")
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedText)
		})
	}
}
