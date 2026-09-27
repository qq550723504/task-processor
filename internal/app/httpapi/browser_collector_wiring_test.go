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
		provider, err := publicAcquisitionProvider(cfg)
		require.NoError(t, err, name)
		require.IsType(t, &a1688.Client{}, provider, name+": must stay on the existing HTTP provider")
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

	provider, err := publicAcquisitionProvider(cfg)
	require.NoError(t, err)
	require.IsType(t, &browserclient.Client{}, provider)
}

// A nil config must not panic and must stay on the existing provider.
func TestPublicAcquisitionProviderHandlesNilConfig(t *testing.T) {
	provider, err := publicAcquisitionProvider(nil)
	require.NoError(t, err)
	require.IsType(t, &a1688.Client{}, provider)
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
