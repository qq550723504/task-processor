package main

import (
	"context"
	"net"
	"testing"
	"time"

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

// The negative startup-quarantine escape hatch exists for tests. A production
// collector must not start with it, or it could immediately reuse an exit IP that
// was challenged just before the restart. A bare "-1" is rejected by the flag
// parser as a malformed duration; a well-formed negative one has to be rejected by
// run itself.
func TestRunRejectsNegativeStartupQuarantine(t *testing.T) {
	withCredential(t)
	for _, tc := range []string{"-1s", "-1m"} {
		err := run(context.Background(), append(baseArgs("chrome.exe"), "-startup-quarantine", tc))
		require.Error(t, err, tc)
		require.Contains(t, err.Error(), "must not be negative", tc)
	}
	err := run(context.Background(), append(baseArgs("chrome.exe"), "-startup-quarantine", "-1"))
	require.Error(t, err)
}

// The quarantine is a duration, so it has to accept the same syntax as the
// duration flags next to it. Reading it as an integer rejected "-startup-quarantine=5m"
// outright.
//
// A bare number is rejected outright rather than reinterpreted: "300" meaning 300
// nanoseconds would look configured while removing restart protection entirely, so
// failing on it is the safe direction.
//
// The listen port is occupied so that run reaches the point of serving and fails
// there: reaching that failure is what proves the flag parsed.
func TestStartupQuarantineAcceptsDurationSyntax(t *testing.T) {
	withCredential(t)
	for _, tc := range []string{"5m", "300s", "1500ms", "1h"} {
		occupied, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := occupied.Addr().String()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = run(ctx, append(baseArgs("chrome.exe"),
			"-startup-quarantine", tc, "-listen", addr))
		cancel()
		occupied.Close()

		if err == nil {
			continue
		}
		require.NotContains(t, err.Error(), "invalid value",
			"%q must parse like the adjacent duration flags", tc)
		require.NotContains(t, err.Error(), "must not be negative",
			"%q is a positive duration and must be accepted", tc)
	}
}

// A bare number is not a duration and must fail loudly rather than being read as
// nanoseconds, which would leave a quarantine that looks configured and protects
// nothing.
func TestStartupQuarantineRejectsBareNumber(t *testing.T) {
	withCredential(t)
	err := run(context.Background(), append(baseArgs("chrome.exe"), "-startup-quarantine", "300"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid value")
}

// Zero still means "follow the configured cooldown" and must not be treated as a
// way to switch the quarantine off.
func TestRunZeroStartupQuarantineIsNotADisableSwitch(t *testing.T) {
	withCredential(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = run(ctx, append(baseArgs("chrome.exe"),
		"-startup-quarantine", "0", "-listen", occupied.Addr().String()))
	if err != nil {
		require.NotContains(t, err.Error(), "must not be negative")
	}
}
