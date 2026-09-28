package zitadel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelfUserFactsCanonicalDatesAndVerifiedSubject(t *testing.T) {
	payload := `{"user":{"id":"user-1","details":{"creationDate":"2026-09-01T02:03:04Z"},"human":{"email":{"email":"recipient@example.test","isEmailVerified":true}}},"lastLogin":"2026-09-27T04:05:06Z"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/auth/v1/users/me", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer user-sentinel", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	client := NewSelfServiceClient(server.URL, server.Client())
	facts, err := client.ReadUserFacts(context.Background(), "user-sentinel", "user-1")
	require.NoError(t, err)
	require.Equal(t, "2026-09-01T02:03:04Z", facts.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"))
	require.NotNil(t, facts.LastLogin)
	require.Equal(t, "2026-09-27T04:05:06Z", facts.LastLogin.Format("2006-01-02T15:04:05Z07:00"))
	require.Equal(t, "recipient@example.test", *facts.Profile.Email)
	require.True(t, *facts.Profile.EmailVerified)
	_, err = client.ReadUserFacts(context.Background(), "user-sentinel", "another-user")
	require.Error(t, err)
}

func TestSelfUserFactsMissingLoginAndMalformedDates(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		valid         bool
	}{
		{"no login", `{"user":{"id":"user-1","details":{"creationDate":"2026-09-01T02:03:04Z"},"human":{}}}`, true},
		{"invalid creation", `{"user":{"id":"user-1","details":{"creationDate":"invalid"},"human":{}}}`, false},
		{"invalid login", `{"user":{"id":"user-1","details":{"creationDate":"2026-09-01T02:03:04Z"},"human":{}},"lastLogin":"invalid"}`, false},
		{"wrong subject", `{"user":{"id":"wrong","details":{"creationDate":"2026-09-01T02:03:04Z"},"human":{}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.payload)) }))
			defer server.Close()
			facts, err := NewSelfServiceClient(server.URL, server.Client()).ReadUserFacts(context.Background(), "token", "user-1")
			if tc.valid {
				require.NoError(t, err)
				require.Nil(t, facts.LastLogin)
			} else {
				require.Error(t, err)
			}
		})
	}
}
