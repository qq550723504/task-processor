package amazon

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"runtime"
	"task-processor/internal/product/dataacquisition"
	"testing"
)

func TestMissingInstalledRuntimeFailsClosed(t *testing.T) {
	c, err := New(Options{ExecutablePath: "missing-browser621", DriverDirectory: "missing-driver621", EnabledSites: []string{"us"}})
	require.NoError(t, err)
	require.ErrorIs(t, c.Ready(context.Background()), dataacquisition.ErrUnavailable)
}

func TestInstalledRuntimeCannotBeReplacedByDriverEnvironment(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "package"), 0700))
	node := "node"
	if runtime.GOOS == "windows" {
		node = "node.exe"
	}
	for _, name := range []string{"browser", node, "package/cli.js"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600))
	}
	t.Setenv("PLAYWRIGHT_NODEJS_PATH", "")
	t.Setenv("PLAYWRIGHT_CLI_PATH", "")
	c, err := New(Options{ExecutablePath: filepath.Join(dir, "browser"), DriverDirectory: dir, EnabledSites: []string{"us"}})
	require.NoError(t, err)
	require.NoError(t, c.Ready(context.Background()))
	t.Setenv("PLAYWRIGHT_NODEJS_PATH", filepath.Join(dir, "other-node"))
	require.ErrorIs(t, c.Ready(context.Background()), dataacquisition.ErrUnavailable)
	t.Setenv("PLAYWRIGHT_NODEJS_PATH", filepath.Join(dir, node))
	t.Setenv("PLAYWRIGHT_CLI_PATH", filepath.Join(dir, "other-cli.js"))
	require.ErrorIs(t, c.Ready(context.Background()), dataacquisition.ErrUnavailable)
}
