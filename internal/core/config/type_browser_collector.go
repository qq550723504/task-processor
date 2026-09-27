package config

import "time"

// BrowserCollectorConfig wires the current-application to the standalone
// anonymous public 1688 browser collector process (architecture design D13).
//
// The collector is opt-in: with an empty Endpoint or an empty Credential the
// application keeps the existing anonymous public HTTP provider unchanged. The
// Credential is a service-to-service credential used only between the
// application and the collector; it is never a user or tenant credential and
// must not be reused from any database, session, or user store.
type BrowserCollectorConfig struct {
	// Endpoint is the collector base URL, for example
	// "http://127.0.0.1:19545". Empty disables the collector path.
	Endpoint string `yaml:"endpoint"`
	// Credential is the shared service credential presented to the collector.
	// Empty disables the collector path.
	Credential string `yaml:"credential"`
	// Timeout bounds one collector RPC call. Zero means the default acquisition
	// timeout.
	Timeout time.Duration `yaml:"timeout"`
}
