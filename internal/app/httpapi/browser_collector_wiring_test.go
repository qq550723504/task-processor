package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/core/config"
	"task-processor/internal/integration/acquisition/a1688"
	"task-processor/internal/integration/acquisition/browserclient"
)

// The collector path is opt-in and all-or-nothing: a partial configuration must
// keep the existing anonymous public HTTP provider rather than half-enable a
// path the operator did not fully configure.
func TestPublicAcquisitionProviderRequiresCompleteOptIn(t *testing.T) {
	cases := map[string]config.BrowserCollectorConfig{
		"unset":            {},
		"endpoint only":    {Endpoint: "http://127.0.0.1:19545"},
		"credential only":  {Credential: "secret"},
		"blank endpoint":   {Endpoint: "   ", Credential: "secret"},
		"blank credential": {Endpoint: "http://127.0.0.1:19545", Credential: "   "},
		"both blank":       {Endpoint: "  ", Credential: "  "},
	}
	for name, collector := range cases {
		cfg := &config.Config{BrowserCollector: collector}
		provider, browserService, err := publicAcquisitionProvider(cfg)
		require.NoError(t, err, name)
		require.IsType(t, &a1688.Client{}, provider, name+": must stay on the existing HTTP provider")
		require.False(t, browserService, name+": must not select the browser service")
		if _, ok := browserCollectorSettingsFrom(cfg); ok {
			t.Fatalf("%s: settings must not resolve without a complete opt-in", name)
		}
	}
}

// A complete opt-in must select the collector RPC client instead.
func TestPublicAcquisitionProviderUsesCollectorWhenFullyConfigured(t *testing.T) {
	cfg := &config.Config{BrowserCollector: config.BrowserCollectorConfig{
		Endpoint:   "http://127.0.0.1:19545",
		Credential: "service-secret",
		Timeout:    45 * time.Second,
	}}
	settings, ok := browserCollectorSettingsFrom(cfg)
	require.True(t, ok)
	require.Equal(t, "http://127.0.0.1:19545", settings.endpoint)
	require.Equal(t, "service-secret", settings.credential)
	require.Equal(t, 45*time.Second, settings.timeout)

	provider, browserService, err := publicAcquisitionProvider(cfg)
	require.NoError(t, err)
	require.IsType(t, &browserclient.Client{}, provider)
	// The configured collector must also select the browser service, otherwise
	// the provider is injected but the generic service still owns the request.
	require.True(t, browserService, "a configured collector must select the browser service")
}

// A nil config must not panic and must stay on the existing provider.
func TestPublicAcquisitionProviderHandlesNilConfig(t *testing.T) {
	provider, browserService, err := publicAcquisitionProvider(nil)
	require.NoError(t, err)
	require.IsType(t, &a1688.Client{}, provider)
	require.False(t, browserService)
}

// A zero timeout must fall back to the default acquisition timeout rather than
// producing an unbounded RPC call.
func TestBrowserCollectorSettingsDefaultTimeout(t *testing.T) {
	cfg := &config.Config{BrowserCollector: config.BrowserCollectorConfig{
		Endpoint:   "http://127.0.0.1:19545",
		Credential: "secret",
	}}
	settings, ok := browserCollectorSettingsFrom(cfg)
	require.True(t, ok)
	require.Positive(t, settings.timeout)
}

// A conventional base URL with a trailing slash must not become a double slash,
// which the collector's exact path match would reject with 404.
func TestBrowserCollectorEndpointIsNormalized(t *testing.T) {
	for _, endpoint := range []string{
		"http://127.0.0.1:19545",
		"http://127.0.0.1:19545/",
		"  http://127.0.0.1:19545/  ",
		"https://collector.internal:8443",
		"https://collector.internal/base",
		"https://collector.internal/base/",
	} {
		cfg := &config.Config{BrowserCollector: config.BrowserCollectorConfig{Endpoint: endpoint, Credential: "secret"}}
		provider, browserService, err := publicAcquisitionProvider(cfg)
		require.NoError(t, err, endpoint)
		require.IsType(t, &browserclient.Client{}, provider, endpoint)
		require.True(t, browserService, endpoint)
	}
	// A base URL carrying a query or fragment carries no meaning for this RPC and
	// is rejected rather than silently truncated.
	for _, endpoint := range []string{"http://127.0.0.1:19545/?a=1", "ftp://collector/x", "not-a-url"} {
		cfg := &config.Config{BrowserCollector: config.BrowserCollectorConfig{Endpoint: endpoint, Credential: "secret"}}
		_, _, err := publicAcquisitionProvider(cfg)
		require.Error(t, err, endpoint)
	}
}
