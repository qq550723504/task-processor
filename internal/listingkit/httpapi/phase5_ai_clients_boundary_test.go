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
	require.NotContains(t, content, "func buildListingKitClientFallback(cfg *config.Config, clientName string) *openaiclient.ClientConfig {")
	require.NotContains(t, content, "func sanitizeListingKitClientFallback(cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {")
	require.NotContains(t, content, "func normalizeListingKitClientName(name string) string {")
	require.NotContains(t, content, "func normalizeListingKitImageSelector(selector string) string {")
	require.NotContains(t, content, "func enforceListingKitImageClientTimeout(clientName string, cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {")
	require.NotContains(t, content, "type strictListingKitChatClient struct {")
	require.NotContains(t, content, "type strictListingKitConfiguredImageClient struct {")
	require.NotContains(t, content, "func resolveStrictListingKitClient(")
	require.NotContains(t, content, "func resolveStrictListingKitImageClient(")
	require.NotContains(t, content, "func buildStrictListingKitChatClient(")
	require.NotContains(t, content, "func buildStrictListingKitImageClient(")
	require.NotContains(t, content, "func buildStrictListingKitNanobananaImageClient(")
	require.NotContains(t, content, "func buildListingKitRoutedImageClient(")

	require.Contains(t, content, "func BuildSheinCategoryLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.TextChatCompleter {")
}

func TestAIClientFallbackHelpersFileOwnsFallbackSanitizingAndNaming(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("ai_client_fallback_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func buildListingKitClientFallback(cfg *config.Config, clientName string) *openaiclient.ClientConfig {")
	require.Contains(t, content, "func sanitizeListingKitClientFallback(cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {")
	require.Contains(t, content, "func normalizeListingKitClientName(name string) string {")
}

func TestAIClientStrictChatFileOwnsStrictChatResolution(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("ai_client_strict_chat.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type strictListingKitChatClient struct {")
	require.Contains(t, content, "func resolveStrictListingKitClient(")
	require.NotContains(t, content, "AnalyzeImage")
}

func TestAIClientBuildersFileOwnsStrictClientConstruction(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("ai_client_builders.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func buildStrictListingKitChatClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) openaiclient.TextChatCompleter {")
	require.NotContains(t, content, "ImageGenerator")
}
