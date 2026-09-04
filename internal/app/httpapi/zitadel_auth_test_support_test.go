package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"task-processor/internal/core/config"
)

const appHTTPTestBearerToken = "app-http-test-token"
const appHTTPTestViewerBearerToken = "app-http-test-viewer-token"

var appHTTPTestRouteAuthorization routeAuthorization
var appHTTPTestConfig *config.Config

func TestMain(m *testing.M) {
	var zitadel *httptest.Server
	zitadel = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"introspection_endpoint": zitadel.URL + "/oauth/v2/introspect"})
		case "/oauth/v2/introspect":
			roles := map[string]any{
				"platform_admin":      map[string]any{},
				"listingkit_admin":    map[string]any{},
				"listingkit_operator": map[string]any{},
			}
			if r.FormValue("token") == appHTTPTestViewerBearerToken {
				roles = map[string]any{"listingkit_viewer": map[string]any{}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active":                                true,
				"sub":                                   "app-http-test-user",
				"user_id":                               "app-http-test-user",
				"urn:zitadel:iam:user:resourceowner:id": "app-http-test-tenant",
				"urn:zitadel:iam:org:project:roles":     roles,
			})
		default:
			http.NotFound(w, r)
		}
	}))

	var err error
	appHTTPTestConfig = &config.Config{ListingKit: config.ListingKitConfig{
		PlatformAdminRoles: []string{"platform_admin"},
		Zitadel: config.ListingKitZitadelConfig{
			IssuerURL: zitadel.URL,
			ClientID:  "app-http-test-client",
		},
	}}
	appHTTPTestRouteAuthorization, err = buildRouteAuthorization(appHTTPTestConfig)
	if err != nil {
		zitadel.Close()
		panic(err)
	}

	code := m.Run()
	zitadel.Close()
	os.Exit(code)
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
