package zitadel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

func TestVerifierClassifiesDependencyFailureSeparatelyFromInvalidToken(t *testing.T) {
	t.Run("dependency transport", func(t *testing.T) {
		verifier := NewVerifier(Config{
			IssuerURL: "https://issuer.example", ClientID: "api",
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial failed")
			})},
		})

		_, err := verifier.Verify(context.Background(), "user-token")

		require.ErrorContains(t, err, "ZITADEL discovery failed:")
		require.ErrorContains(t, err, "dial failed")
		require.True(t, IsVerificationDependencyUnavailable(err))
		require.False(t, IsVerificationInvalid(err))
	})

	t.Run("inactive token", func(t *testing.T) {
		server := newAuthServer(t, map[string]any{"active": false})
		defer server.Close()
		verifier := NewVerifier(Config{IssuerURL: server.URL, ClientID: "api", HTTPClient: server.Client()})

		_, err := verifier.Verify(context.Background(), "user-token")

		require.ErrorContains(t, err, "inactive token")
		require.True(t, IsVerificationInvalid(err))
		require.False(t, IsVerificationDependencyUnavailable(err))
	})
}

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
				"urn:zitadel:iam:org:project:project-1:roles": []any{
					map[string]any{"listingkit_operator": map[string]any{"displayName": "Operator"}},
				},
				"urn:zitadel:iam:org:project:other-project:roles": []any{
					map[string]any{"foreign_admin": map[string]any{}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	verifier := NewVerifier(Config{
		IssuerURL: server.URL, ClientID: "api", ClientSecret: "secret",
		ProjectID: " project-1 ", HTTPClient: server.Client(),
	})

	for range 2 {
		got, err := verifier.Verify(context.Background(), "user-token")
		require.NoError(t, err)
		require.Equal(t, authidentity.AuthenticatedIdentity{
			TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"}, HomeOrganizationID: "org-1",
		}, got)
	}

	require.Equal(t, int32(1), discoveryHits.Load())
	require.Equal(t, int32(2), introspectionHits.Load())
}

func TestVerifierRejectsExpiredActiveToken(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	server := newAuthServer(t, map[string]any{
		"active":                                true,
		"sub":                                   "user-1",
		"urn:zitadel:iam:user:resourceowner:id": "org-1",
		"exp":                                   now.Add(-time.Second).Unix(),
	})
	defer server.Close()

	verifier := newVerifier(normalizeConfig(Config{
		IssuerURL:  server.URL,
		ClientID:   "api",
		HTTPClient: server.Client(),
	}))
	verifier.now = func() time.Time { return now }

	_, err := verifier.Verify(context.Background(), "user-token")

	require.ErrorContains(t, err, "expired")
}

func TestVerifierCopiesFutureExpiryIntoCanonicalIdentity(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(15 * time.Minute)
	server := newAuthServer(t, map[string]any{
		"active":                                true,
		"sub":                                   " user-1 ",
		"urn:zitadel:iam:user:resourceowner:id": " org-home ",
		"exp":                                   expiresAt.Unix(),
		"urn:zitadel:iam:org:project:project-1:roles": []any{
			map[string]any{"listingkit_operator": map[string]any{}},
		},
	})
	defer server.Close()

	verifier := newVerifier(normalizeConfig(Config{
		IssuerURL:  server.URL,
		ClientID:   "api",
		ProjectID:  "project-1",
		HTTPClient: server.Client(),
	}))
	verifier.now = func() time.Time { return now }

	got, err := verifier.Verify(context.Background(), "user-token")

	require.NoError(t, err)
	require.Equal(t, authidentity.AuthenticatedIdentity{
		TenantID:           "org-home",
		UserID:             "user-1",
		Roles:              []string{"listingkit_operator"},
		HomeOrganizationID: "org-home",
		TokenExpiresAt:     expiresAt,
	}, got)
}

func TestParseRolesForProjectSupportsDynamicArrayRoleMaps(t *testing.T) {
	roles := ParseRolesForProject([]byte(`{
		"urn:zitadel:iam:org:project:project-1:roles": [
			{"listingkit_operator": {"displayName": "Operator"}},
			{"listingkit_admin": {}}
		],
		"urn:zitadel:iam:org:project:other-project:roles": [
			{"foreign_admin": {}}
		]
	}`), "project-1")

	require.Equal(t, []string{"listingkit_operator", "listingkit_admin"}, roles)
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
