package zitadel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelfServiceClientUsesAuthenticatedUserAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/auth/v1/users/me/email", r.URL.Path)
		require.Equal(t, "Bearer user-token", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, map[string]string{"email": "user@example.test"}, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSelfServiceClient(server.URL, server.Client())
	err := client.Execute(context.Background(), "user-token", SelfServiceSetEmail, []byte(`{"email":"user@example.test"}`))
	require.NoError(t, err)
}

func TestSelfServiceClientReadsOfficialProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/auth/v1/users/me/profile", r.URL.Path)
		require.Equal(t, "Bearer user-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"profile":{"firstName":"First","lastName":"Last","displayName":"Name","gender":"GENDER_UNSPECIFIED"}}`))
	}))
	defer server.Close()

	profile, err := NewSelfServiceClient(server.URL, server.Client()).ReadProfile(context.Background(), "user-token")
	require.NoError(t, err)
	require.Equal(t, SelfServiceProfile{FirstName: "First", LastName: "Last", DisplayName: "Name", Gender: "GENDER_UNSPECIFIED"}, profile)
}

func TestSelfServiceClientMapsOfficialOperations(t *testing.T) {
	tests := []struct {
		operation SelfServiceOperation
		method    string
		path      string
	}{
		{SelfServiceSetPhone, http.MethodPut, "/auth/v1/users/me/phone"},
		{SelfServiceResendEmailVerification, http.MethodPost, "/auth/v1/users/me/email/_resend_verification"},
		{SelfServiceVerifyEmail, http.MethodPost, "/auth/v1/users/me/email/_verify"},
		{SelfServiceResendPhoneVerification, http.MethodPost, "/auth/v1/users/me/phone/_resend_verification"},
		{SelfServiceVerifyPhone, http.MethodPost, "/auth/v1/users/me/phone/_verify"},
		{SelfServiceUpdatePassword, http.MethodPut, "/auth/v1/users/me/password"},
	}
	for _, tt := range tests {
		t.Run(string(tt.operation), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, tt.method, r.Method)
				require.Equal(t, tt.path, r.URL.Path)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			require.NoError(t, NewSelfServiceClient(server.URL, server.Client()).Execute(context.Background(), "token", tt.operation, []byte(`{}`)))
		})
	}
}

func TestSelfServiceClientFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		token  string
		op     SelfServiceOperation
	}{
		{name: "missing issuer", issuer: "", token: "token", op: SelfServiceSetEmail},
		{name: "unsafe issuer", issuer: "https://issuer.example/?redirect=https://evil.example", token: "token", op: SelfServiceSetEmail},
		{name: "missing token", issuer: "https://issuer.example", token: "", op: SelfServiceSetEmail},
		{name: "unknown operation", issuer: "https://issuer.example", token: "token", op: SelfServiceOperation("unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSelfServiceClient(tt.issuer, http.DefaultClient)
			err := client.Execute(context.Background(), tt.token, tt.op, []byte(`{}`))
			require.Error(t, err)
		})
	}
}

func TestSelfServiceClientReturnsUpstreamStatusWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"do not expose this"}`))
	}))
	defer server.Close()

	err := NewSelfServiceClient(server.URL, server.Client()).Execute(context.Background(), "token", SelfServiceSetEmail, []byte(`{"email":"bad"}`))
	var upstreamErr *SelfServiceError
	require.ErrorAs(t, err, &upstreamErr)
	require.Equal(t, http.StatusBadRequest, upstreamErr.StatusCode)
	require.NotContains(t, err.Error(), "do not expose this")
}
