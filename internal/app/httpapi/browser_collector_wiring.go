package httpapi

import (
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/integration/acquisition/a1688"
	"task-processor/internal/integration/acquisition/browserclient"
	"task-processor/internal/integration/acquisition/browsercollector"
	"task-processor/internal/product/sourcing"
)

// Browser collector wiring (architecture design D13, decisions A1/A2/A6).
//
// The production provider is selected by configuration, never by ambient
// state: an operator must opt in by supplying both the collector endpoint and
// the service credential. When they are absent the existing anonymous public
// HTTP provider is used unchanged, so turning the collector on is an explicit,
// reversible deployment action rather than a code-path accident.

// browserCollectorSettings holds the resolved, config-gated collector wiring.
type browserCollectorSettings struct {
	endpoint   string
	credential string
	timeout    time.Duration
}

// browserCollectorSettingsFrom resolves the collector wiring from raw
// configuration. It returns ok=false when the operator has not opted in, which
// keeps the previous provider in place.
func browserCollectorSettingsFrom(cfg *config.Config) (browserCollectorSettings, bool) {
	if cfg == nil {
		return browserCollectorSettings{}, false
	}
	collector := cfg.BrowserCollector
	endpoint := strings.TrimSpace(collector.Endpoint)
	credential := strings.TrimSpace(collector.Credential)
	// Both halves are required: an endpoint without a credential would start a
	// client that can never be admitted, and a credential without an endpoint
	// has nothing to call. Either way, stay on the existing provider.
	if endpoint == "" || credential == "" {
		return browserCollectorSettings{}, false
	}
	timeout := collector.Timeout
	if timeout <= 0 {
		timeout = sourcing.AcquisitionTimeout
	}
	return browserCollectorSettings{endpoint: endpoint, credential: credential, timeout: timeout}, true
}

// publicAcquisitionProvider builds the anonymous public acquisition provider for
// the current composition. It prefers the browser collector when the deployment
// has explicitly configured it, and otherwise returns the existing HTTP provider.
func publicAcquisitionProvider(cfg *config.Config) (sourcing.PublicAcquirer, bool, error) {
	settings, ok := browserCollectorSettingsFrom(cfg)
	if !ok {
		provider, err := publicHTTPProvider()
		return provider, false, err
	}
	// The client-side admission is fail-closed: a collector that rejects the
	// credential surfaces as an availability failure, never as a fabricated
	// product and never as a silent fallback to weaker collection.
	client, err := browserclient.New(browserclient.Options{
		Endpoint:  settings.endpoint,
		Admission: browsercollector.AttachSharedSecret(settings.credential),
		Timeout:   settings.timeout,
	})
	if err != nil {
		return nil, false, err
	}
	// The second result reports that the browser provider is in use, so the
	// caller can build the browser service rather than the generic one. Without
	// it the provider would be injected but the generic service would still own
	// the request, bypassing replay-first, StartPrepared and the provider
	// child budget.
	return client, true, nil
}

// publicHTTPProvider is the existing anonymous public HTTP provider.
func publicHTTPProvider() (sourcing.PublicAcquirer, error) { return a1688.New(), nil }
