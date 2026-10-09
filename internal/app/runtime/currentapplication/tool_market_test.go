package currentapplication

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestToolMarketInstallReleaseMustMatchExactReceiver(t *testing.T) {
	dir := t.TempDir()
	c := &ToolMarketConfig{CaptureAppURL: "https://localhost:31544/capture/1688", PackageDirectory: dir}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "release.json"), []byte(`{"schemaVersion":1,"captureAppUrl":"https://localhost:31444/capture/1688","sha256":"untrusted","filename":"shuomi-1688-capture.zip"}`), 0600))
	require.Nil(t, c.PackageConfig(), "same hostname with a different port is not this installation")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "release.json"), []byte(`{"schemaVersion":1,"captureAppUrl":"https://localhost:31544/capture/1688","sha256":"recorded","filename":"../../other.zip"}`), 0600))
	require.Nil(t, c.PackageConfig())
}
func TestToolMarketRequiresExistingRestrictedDedicatedOwner(t *testing.T) {
	cfg := runtimeTestConfig()
	cfg.ToolMarket = &ToolMarketConfig{Database: DatabaseConfig{Host: "127.0.0.1", Port: 5433, User: "tool_market_runtime", Password: "private-password", Database: "tool_market", MaxConnections: 4}, CaptureAppURL: "https://localhost:31544/capture/1688"}
	if cfg.Referrals.Enabled {
		cfg.Referrals.PublicAppOrigin = "https://localhost:31544"
	}
	require.NoError(t, cfg.validateToolMarket())
	cfg.ToolMarket.Database.User = "tool_market_owner"
	require.Error(t, cfg.validateToolMarket())
	cfg.ToolMarket.Database.User = "tool_market_runtime"
	cfg.ToolMarket.Database.Database = "product_acquisition"
	require.Error(t, cfg.validateToolMarket())
}
