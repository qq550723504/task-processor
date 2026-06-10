package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIClientsFileStaysFocusedOnClientBuilderAndResolverAssembly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("ai_clients.go")
	require.NoError(t, err)
	content := string(src)

	require.NotContains(t, content, "type listingKitRoutedImageClient struct {")
	require.NotContains(t, content, "func normalizeListingKitImageSelector(selector string) string {")
	require.NotContains(t, content, "func enforceListingKitImageClientTimeout(clientName string, cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {")

	require.Contains(t, content, "func BuildStudioImageGenerator(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {")
	require.Contains(t, content, "func resolveStrictListingKitImageClient(")
}

func TestAIClientImageRoutingHelpersFileOwnsRoutedImageLogic(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("ai_client_image_routing.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type listingKitRoutedImageClient struct {")
	require.Contains(t, content, "func normalizeListingKitImageSelector(selector string) string {")
	require.Contains(t, content, "func enforceListingKitImageClientTimeout(clientName string, cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {")
}
