package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func baseArgs(browserPath string) []string {
	return []string{"-browser", browserPath, "-credential", "test-credential"}
}

// The collector must refuse to start without a caller-admission credential: an
// unadmitted collector would let any workload on the same network drive
// Chromium and spend the shared IP budget (design A1 / D13 / finding #14).
func TestRunRefusesToStartWithoutAdmissionCredential(t *testing.T) {
	t.Setenv(credentialEnvKey, "")
	err := run(context.Background(), []string{"-browser", "chrome.exe"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "caller admission credential is required")
}

// An unbounded egress allowlist is never acceptable: the collector must always
// have an explicit origin set (D12).
func TestRunRefusesEmptyOriginAllowlist(t *testing.T) {
	err := run(context.Background(), append(baseArgs("chrome.exe"), "-allowed-origins", " , "))
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be empty")
}

// A missing browser path is a configuration error, not a silent no-op.
func TestRunRequiresBrowserPath(t *testing.T) {
	err := run(context.Background(), []string{"-credential", "x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "-browser is required")
}

func TestSplitOriginsTrimsAndDropsBlanks(t *testing.T) {
	require.Equal(t, []string{"https://a.test", "https://b.test"}, splitOrigins(" https://a.test , , https://b.test "))
	require.Empty(t, splitOrigins("  ,  "))
	require.Empty(t, splitOrigins(""))
}
