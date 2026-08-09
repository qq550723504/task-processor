package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"task-processor/internal/core/config"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

const appHTTPTestBearerToken = "app-http-test-token"

func TestMain(m *testing.M) {
	var zitadel *httptest.Server
	zitadel = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"introspection_endpoint": zitadel.URL + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "app-http-test-user",
				"user_id":                               "app-http-test-user",
				"urn:zitadel:iam:user:resourceowner:id": "app-http-test-tenant",
				"urn:zitadel:iam:org:project:roles": map[string]any{
					"platform_admin":      map[string]any{},
					"listingkit_admin":    map[string]any{},
					"listingkit_operator": map[string]any{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	issuerBefore, issuerSet := os.LookupEnv("ZITADEL_ISSUER_URL")
	clientBefore, clientSet := os.LookupEnv("ZITADEL_CLIENT_ID")
	_ = os.Setenv("ZITADEL_ISSUER_URL", zitadel.URL)
	_ = os.Setenv("ZITADEL_CLIENT_ID", "app-http-test-client")
	listingkithttpapi.ConfigureListingKitZitadelAuth(config.ListingKitZitadelConfig{
		IssuerURL: zitadel.URL,
		ClientID:  "app-http-test-client",
	})
	_ = listingkithttpapi.ConfigureListingKitAuthorization(nil, []string{"platform_admin"})

	code := m.Run()
	zitadel.Close()
	restoreTestEnv("ZITADEL_ISSUER_URL", issuerBefore, issuerSet)
	restoreTestEnv("ZITADEL_CLIENT_ID", clientBefore, clientSet)
	os.Exit(code)
}

func restoreTestEnv(key, value string, wasSet bool) {
	if wasSet {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}

func withAppHTTPTestBearer(request *http.Request) *http.Request {
	request.Header.Set("Authorization", "Bearer "+appHTTPTestBearerToken)
	return request
}

func authenticatedAppHTTPTestClient(client *http.Client) *http.Client {
	clone := *client
	transport := clone.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = appHTTPTestBearerTransport{base: transport}
	return &clone
}

type appHTTPTestBearerTransport struct {
	base http.RoundTripper
}

func (t appHTTPTestBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	withAppHTTPTestBearer(clone)
	return t.base.RoundTrip(clone)
}
