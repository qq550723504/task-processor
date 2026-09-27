package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func baseArgs(browserPath string) []string {
	return []string{"-browser", browserPath}
}

// withCredential sets the service credential the way a deployment must: through
// the environment, never through a command-line flag.
func withCredential(t *testing.T) {
	t.Helper()
	t.Setenv(credentialEnvKey, "test-service-credential")
}

// The collector must refuse to start without a caller-admission credential: an
// unadmitted collector would let any workload on the same network drive
// Chromium and spend the shared IP budget (design A1 / D13 / finding #14).
func TestRunRefusesToStartWithoutAdmissionCredential(t *testing.T) {
	t.Setenv(credentialEnvKey, "")
	err := run(context.Background(), []string{"-browser", "chrome.exe"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "caller admission credential is required")

	// The credential must not be accepted from the command line, where it would
	// be visible in the process table, container specs and shell history.
	t.Setenv(credentialEnvKey, "")
	err = run(context.Background(), []string{"-browser", "chrome.exe", "-credential", "secret"})
	require.Error(t, err, "-credential must not be an accepted flag")
}

// An unbounded egress allowlist is never acceptable: the collector must always
// have an explicit origin set (D12).
func TestRunRefusesEmptyOriginAllowlist(t *testing.T) {
	withCredential(t)
	err := run(context.Background(), append(baseArgs("chrome.exe"), "-allowed-origins", " , "))
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be empty")
}

// A missing browser path is a configuration error, not a silent no-op.
func TestRunRequiresBrowserPath(t *testing.T) {
	err := run(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "-browser is required")
}

func TestSplitOriginsTrimsAndDropsBlanks(t *testing.T) {
	require.Equal(t, []string{"https://a.test", "https://b.test"}, splitOrigins(" https://a.test , , https://b.test "))
	require.Empty(t, splitOrigins("  ,  "))
	require.Empty(t, splitOrigins(""))
}
