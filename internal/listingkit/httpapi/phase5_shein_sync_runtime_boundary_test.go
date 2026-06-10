package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSheinSyncRuntimeFileStaysFocusedOnServiceAssembly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_sync_runtime.go")
	require.NoError(t, err)
	content := string(src)

	require.NotContains(t, content, "type sheinPromotionBridgeRuntimeFactory struct {")
	require.NotContains(t, content, "func sheinRuntimeTenantID(ctx context.Context) (int64, error) {")

	require.Contains(t, content, "func buildSheinSyncRuntimeServices(")
	require.Contains(t, content, "func buildSheinPromotionStrategyProvider(")
}

func TestSheinSyncRuntimeBridgeHelpersFileOwnsPromotionBridgeShaping(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_sync_runtime_bridge_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type sheinPromotionBridgeRuntimeFactory struct {")
	require.Contains(t, content, "func (f sheinPromotionBridgeRuntimeFactory) BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error) {")
	require.Contains(t, content, "func sheinRuntimeTenantID(ctx context.Context) (int64, error) {")
}
