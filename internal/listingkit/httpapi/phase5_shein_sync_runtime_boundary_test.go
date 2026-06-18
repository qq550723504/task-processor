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
	require.NotContains(t, content, "sheinPromotionBridgeRuntimeFactory{")
	require.NotContains(t, content, "sheinManagementStoreCatalog{repo:")
	require.NotContains(t, content, "SheinAPIClientFactoryBuilder(")
	require.NotContains(t, content, "func sheinRuntimeTenantID(ctx context.Context) (int64, error) {")
	require.NotContains(t, content, "type localManagementPromotionStrategyProvider struct {")
	require.NotContains(t, content, "func buildSheinPromotionStrategyProvider(input BuildServiceInput, closers *closerStack) (localManagementPromotionStrategyProvider, error) {")

	require.Contains(t, content, "func buildSheinSyncRuntimeServices(")
	require.Contains(t, content, "strategyProvider, err := buildSheinPromotionStrategyProvider(input, closers)")
	require.Contains(t, content, "bridgeFactory := buildSheinPromotionBridgeRuntimeFactory(input, repositories)")
}

func TestSheinSyncRuntimeBridgeHelpersFileOwnsPromotionBridgeShaping(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_sync_runtime_bridge_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type sheinPromotionBridgeRuntimeFactory struct {")
	require.Contains(t, content, "func buildSheinPromotionBridgeRuntimeFactory(input BuildServiceInput, repositories *builtRepositories) sheinPromotionBridgeRuntimeFactory {")
	require.Contains(t, content, "func (f sheinPromotionBridgeRuntimeFactory) BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error) {")
	require.Contains(t, content, "func sheinRuntimeTenantID(ctx context.Context) (int64, error) {")
}

func TestSheinSyncRuntimeStrategyHelpersFileOwnsManagementStrategyAssembly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_sync_runtime_strategy_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type localManagementPromotionStrategyProvider struct {")
	require.Contains(t, content, "func (p localManagementPromotionStrategyProvider) GetPromotionStrategy(_ context.Context, storeID int64, _ string) (*sheinsync.SheinPromotionStrategy, error) {")
	require.Contains(t, content, "func buildSheinPromotionStrategyProvider(input BuildServiceInput, closers *closerStack) (localManagementPromotionStrategyProvider, error) {")
}
