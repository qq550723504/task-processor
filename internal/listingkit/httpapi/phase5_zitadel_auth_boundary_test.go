package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZitadelAuthFileStaysFocusedOnMiddlewareAndRuntimeConfig(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth.go")
	require.NoError(t, err)
	content := string(src)

	require.NotContains(t, content, "func firstNonEmptyZitadelValue(values ...string) string {")
	require.NotContains(t, content, "func parseZitadelRoles(data []byte) []string {")
	require.NotContains(t, content, "func stringSliceToSet(values []string) map[string]struct{} {")
	require.NotContains(t, content, "func valueInSet(value string, set map[string]struct{}) bool {")

	require.Contains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.Contains(t, content, "func NewZitadelAuthMiddlewareFromEnv() gin.HandlerFunc {")
	require.Contains(t, content, "func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {")
}

func TestZitadelAuthParsingHelpersFileOwnsParsingAndSetHelpers(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_parsing_helpers.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func firstNonEmptyZitadelValue(values ...string) string {")
	require.Contains(t, content, "func parseZitadelRoles(data []byte) []string {")
	require.Contains(t, content, "func stringSliceToSet(values []string) map[string]struct{} {")
	require.Contains(t, content, "func valueInSet(value string, set map[string]struct{}) bool {")
}
