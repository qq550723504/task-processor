package deviceauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthorizeRejectsCrossOriginTokenEndpoint(t *testing.T) {
	issuer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = w.Write([]byte(`{"device_authorization_endpoint":"` + issuerURLPlaceholder + `/device","token_endpoint":"https://elsewhere.example/token"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer issuer.Close()
	// Replace the placeholder after the server has a stable URL.
	issuer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = w.Write([]byte(`{"device_authorization_endpoint":"` + issuer.URL + `/device","token_endpoint":"https://elsewhere.example/token"}`))
			return
		}
		http.NotFound(w, r)
	})

	_, err := Authorize(context.Background(), Config{IssuerURL: issuer.URL, ClientID: "client", ProjectID: "project", Timeout: time.Second, HTTPClient: issuer.Client()}, recordingPresenter{})
	require.ErrorContains(t, err, "same origin")
}

func TestSameOriginEndpointUsesEffectiveDefaultPort(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		uri    string
		wantOK bool
	}{
		{name: "issuer omits https default port", issuer: "https://issuer.example", uri: "https://issuer.example:443/token", wantOK: true},
		{name: "endpoint omits https default port", issuer: "https://issuer.example:443", uri: "https://issuer.example/token", wantOK: true},
		{name: "non-default port remains distinct", issuer: "https://issuer.example:8443", uri: "https://issuer.example/token", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issuer, err := url.Parse(test.issuer)
			require.NoError(t, err)

			_, err = sameOriginEndpoint(issuer, test.uri, "token endpoint")
			if test.wantOK {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "same origin")
			}
		})
	}
}

func TestAuthorizeRejectsOfflineAccessScope(t *testing.T) {
	_, err := Authorize(context.Background(), Config{IssuerURL: "https://issuer.example", ClientID: "client", ProjectID: "project", Scopes: "openid offline_access"}, recordingPresenter{})
	require.ErrorContains(t, err, "offline_access")
}

func TestAuthorizeRejectsOfflineAccessScopeAcrossWhitespace(t *testing.T) {
	for _, scopes := range []string{"openid\toffline_access", "openid\noffline_access", "openid\r\noffline_access"} {
		_, err := Authorize(context.Background(), Config{IssuerURL: "https://issuer.example", ClientID: "client", ProjectID: "project", Scopes: scopes}, recordingPresenter{})
		require.ErrorContains(t, err, "offline_access")
	}
}

func TestOAuthScopesIncludeAdminAlias(t *testing.T) {
	scopes, err := oauthScopes("", "project")
	require.NoError(t, err)
	require.Contains(t, scopes, "urn:zitadel:iam:org:project:role:admin")
}

func TestAuthorizeHandlesPendingHTTPErrorThenApproved(t *testing.T) {
	tokenCalls := 0
	server := httptest.NewTLSServer(nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"device_authorization_endpoint": server.URL + "/device", "token_endpoint": server.URL + "/token"})
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "device", "user_code": "ABCD", "verification_uri": server.URL + "/verify", "expires_in": 30, "interval": 1})
		case "/token":
			tokenCalls++
			if tokenCalls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access"})
		default:
			http.NotFound(w, r)
		}
	})
	defer server.Close()

	token, err := Authorize(context.Background(), Config{IssuerURL: server.URL, ClientID: "client", ProjectID: "project", Timeout: time.Second, HTTPClient: server.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}, recordingPresenter{})
	require.NoError(t, err)
	require.Equal(t, "access", token)
	require.Equal(t, 2, tokenCalls)
}

func TestAuthorizeRejectsRefreshToken(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"device_authorization_endpoint": server.URL + "/device", "token_endpoint": server.URL + "/token"})
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "device", "user_code": "ABCD", "verification_uri": server.URL + "/verify", "expires_in": 30})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"refresh_token": "secret"})
		}
	})
	defer server.Close()

	_, err := Authorize(context.Background(), Config{IssuerURL: server.URL, ClientID: "client", ProjectID: "project", Timeout: time.Second, HTTPClient: server.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}, recordingPresenter{})
	require.ErrorContains(t, err, "refresh token")
}

type recordingPresenter struct{}

func (recordingPresenter) Show(string, string) error { return nil }

const issuerURLPlaceholder = "https://issuer.invalid"
