package currentapplication

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyIdentityProviderRequiresCurrentSameOriginDiscovery(t *testing.T) {
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"issuer":"` + origin + `","authorization_endpoint":"` + origin + `/oauth/v2/authorize","token_endpoint":"` + origin + `/oauth/v2/token","userinfo_endpoint":"` + origin + `/oidc/v1/userinfo","introspection_endpoint":"` + origin + `/oauth/v2/introspect"}`))
	}))
	origin = server.URL
	defer server.Close()
	cfg := IdentityConfig{IssuerURL: origin, AuthorizationAPIURL: origin}
	if err := VerifyIdentityProvider(context.Background(), cfg); err != nil {
		t.Fatalf("VerifyIdentityProvider() error = %v", err)
	}
}

func TestVerifyIdentityProviderRejectsUnavailableOrForeignDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"issuer":"http://127.0.0.1:1","authorization_endpoint":"http://127.0.0.1:1/auth","token_endpoint":"http://127.0.0.1:1/token","userinfo_endpoint":"http://127.0.0.1:1/user","introspection_endpoint":"http://127.0.0.1:1/introspect"}`))
	}))
	cfg := IdentityConfig{IssuerURL: server.URL, AuthorizationAPIURL: server.URL}
	if err := VerifyIdentityProvider(context.Background(), cfg); err == nil {
		t.Fatal("foreign discovery was accepted")
	}
	server.Close()
	if err := VerifyIdentityProvider(context.Background(), cfg); err == nil {
		t.Fatal("unavailable provider was accepted")
	}
}
