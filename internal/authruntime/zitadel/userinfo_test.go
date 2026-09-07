package zitadel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserInfoReadsOnlyVerifiedSelf(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		fail       bool
	}{
		{"profile", `{"sub":"u1","name":"Alice","email":"a@example.test","email_verified":true}`, 200, false},
		{"missing scopes", `{"sub":"u1"}`, 200, false},
		{"technical email", `{"sub":"u1","email":" U-X@PHONE.INVALID ","email_verified":true}`, 200, false},
		{"other subject", `{"sub":"u2","email":"private@example.test"}`, 200, true},
		{"wrong type", `{"sub":"u1","name":3}`, 200, true},
		{"oversize", `{"sub":"u1","name":"` + strings.Repeat("x", 17000) + `"}`, 200, true},
		{"revoked", `{}`, 401, true}, {"denied", `{}`, 403, true}, {"failure", `{}`, 503, true},
		{"redirect", `{}`, 302, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/oidc/v1/userinfo", r.URL.Path)
				require.Equal(t, "Bearer fixture-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			got, err := NewUserInfoClient(server.URL, server.Client()).ReadSelf(context.Background(), "fixture-token", "u1")
			if tc.fail {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private@example.test")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "u1", got.UserID)
			if tc.name == "profile" {
				require.Equal(t, "Alice", *got.DisplayName)
				require.True(t, *got.EmailVerified)
			}
			if tc.name == "technical email" || tc.name == "missing scopes" {
				require.Nil(t, got.Email)
				require.Nil(t, got.EmailVerified)
			}
		})
	}
}

func TestUserInfoPreservesIssuerPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/auth/oidc/v1/userinfo", r.URL.Path)
		_, _ = w.Write([]byte(`{"sub":"u1"}`))
	}))
	defer server.Close()

	profile, err := NewUserInfoClient(server.URL+"/auth/", server.Client()).ReadSelf(context.Background(), "fixture-token", "u1")

	require.NoError(t, err)
	require.Equal(t, "u1", profile.UserID)
}

func TestUserInfoRejectsUnsafeIssuer(t *testing.T) {
	for _, issuer := range []string{
		"ftp://issuer.example/auth",
		"https://user@issuer.example/auth",
		"https://issuer.example/auth?tenant=a",
		"https://issuer.example/auth#fragment",
	} {
		t.Run(issuer, func(t *testing.T) {
			_, err := NewUserInfoClient(issuer, http.DefaultClient).ReadSelf(context.Background(), "fixture-token", "u1")
			require.Error(t, err)
		})
	}
}
