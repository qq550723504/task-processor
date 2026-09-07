package zitadel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

func TestVerifierDiscoveryWaitHonorsCallerDeadline(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	v := NewVerifier(Config{IssuerURL: server.URL, ClientID: "fixture", HTTPClient: server.Client()})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = v.Verify(context.Background(), "first") }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	second := make(chan error, 1)
	go func() { _, err := v.Verify(ctx, "second"); second <- err }()
	select {
	case err := <-second:
		require.Error(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Error("discovery lock ignored caller deadline")
	}
	close(release)
	<-done
}

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

func TestVerifierRejectsOversizedIntrospectionResponseBeforeDecoding(t *testing.T) {
	server := newAuthServer(t, map[string]any{
		"active":                                true,
		"sub":                                   "user-1",
		"urn:zitadel:iam:user:resourceowner:id": "org-1",
		"padding":                               strings.Repeat("x", (1<<20)+1),
	})
	defer server.Close()

	verifier := NewVerifier(Config{
		IssuerURL:  server.URL,
		ClientID:   "api",
		HTTPClient: server.Client(),
	})

	_, err := verifier.Verify(context.Background(), "user-token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "response is too large")
	require.True(t, IsVerificationDependencyUnavailable(err))
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

func TestVerifierRejectsMalformedOrOversizedIdentityClaims(t *testing.T) {
	for _, tc := range []struct {
		name            string
		subject         string
		resourceOwnerID string
	}{
		{name: "malformed subject", subject: "/invalid", resourceOwnerID: "org-1"},
		{name: "oversized subject", subject: strings.Repeat("u", 129), resourceOwnerID: "org-1"},
		{name: "malformed resource owner", subject: "user-1", resourceOwnerID: "/invalid"},
		{name: "oversized resource owner", subject: "user-1", resourceOwnerID: strings.Repeat("o", 129)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newAuthServer(t, map[string]any{
				"active":                                true,
				"sub":                                   tc.subject,
				"urn:zitadel:iam:user:resourceowner:id": tc.resourceOwnerID,
			})
			defer server.Close()
			verifier := NewVerifier(Config{IssuerURL: server.URL, ClientID: "api", HTTPClient: server.Client()})

			_, err := verifier.Verify(context.Background(), "user-token")

			require.Error(t, err)
			require.NotContains(t, err.Error(), tc.subject)
			require.NotContains(t, err.Error(), tc.resourceOwnerID)
			require.False(t, IsVerificationInvalid(err))
			require.False(t, IsVerificationDependencyUnavailable(err))
			require.True(t, IsVerificationInvalidResponse(err))
		})
	}
}
