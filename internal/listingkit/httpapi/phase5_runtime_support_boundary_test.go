package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSheinRuntimeSupportFileStaysFocusedOnAdapterConstruction(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_shein.go")
	require.NoError(t, err)
	content := string(src)

	require.NotContains(t, content, "func normalizeSheinCookiePayload(raw string) (string, error) {")
	require.NotContains(t, content, "func tenantIDFromContext(ctx context.Context) int64 {")
	require.NotContains(t, content, "func toSheinClientStoreConfig(storeInfo *listingkit.SheinStoreInfo) *listingkit.SheinRuntimeStoreConfig {")
	require.NotContains(t, content, "func toSheinClientStoreConfigFromListingAdmin(store *listingadmin.Store) *listingkit.SheinRuntimeStoreConfig {")

	require.Contains(t, content, "func buildListingKitSheinCategoryResolver(")
	require.Contains(t, content, "func buildListingKitPromotionRegistrationBridge(apiClient *listingkit.SheinRuntimeAPIClient) activity.PromotionRegistrationBridge {")
}

func TestSheinRuntimeSupportAdapterHelpersFileOwnsRuntimeShapingHelpers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_shein_adapter_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func normalizeSheinCookiePayload(raw string) (string, error) {")
	require.Contains(t, content, "func tenantIDFromContext(ctx context.Context) int64 {")
	require.Contains(t, content, "func toSheinClientStoreConfig(storeInfo *listingkit.SheinStoreInfo) *listingkit.SheinRuntimeStoreConfig {")
	require.Contains(t, content, "func toSheinClientStoreConfigFromListingAdmin(store *listingadmin.Store) *listingkit.SheinRuntimeStoreConfig {")
}
