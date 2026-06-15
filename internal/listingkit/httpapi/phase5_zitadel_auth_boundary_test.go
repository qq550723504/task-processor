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

	require.Contains(t, content, "type zitadelAuthConfig struct {")
	require.Contains(t, content, "type zitadelAuthMiddleware struct {")
	require.Contains(t, content, "type listingKitZitadelRuntimeConfig struct {")

	require.NotContains(t, content, "func firstNonEmptyZitadelValue(values ...string) string {")
	require.NotContains(t, content, "func parseZitadelRoles(data []byte) []string {")
	require.NotContains(t, content, "func stringSliceToSet(values []string) map[string]struct{} {")
	require.NotContains(t, content, "func valueInSet(value string, set map[string]struct{}) bool {")
	require.NotContains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.NotContains(t, content, "func (m *zitadelAuthMiddleware) Handle(c *gin.Context) {")
	require.NotContains(t, content, "func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {")
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

func TestZitadelAuthRuntimeFileOwnsRuntimeConfigAndMiddlewareFactory(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_runtime.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.Contains(t, content, "func ConfigureListingKitAuthorization(platformAdminUsers []string, platformAdminRoles []string) error {")
	require.Contains(t, content, "func NewZitadelAuthMiddlewareFromEnv() gin.HandlerFunc {")
	require.NotContains(t, content, "func (m *zitadelAuthMiddleware) Handle(c *gin.Context) {")
	require.NotContains(t, content, "func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {")
}

func TestZitadelAuthMiddlewareFileOwnsDiscoveryAndTokenVerification(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_middleware.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func (m *zitadelAuthMiddleware) Handle(c *gin.Context) {")
	require.Contains(t, content, "func (m *zitadelAuthMiddleware) verifyToken(r *http.Request, token string) (*zitadelIntrospectionResponse, error) {")
	require.Contains(t, content, "func (m *zitadelAuthMiddleware) getDiscovery(r *http.Request) (zitadelDiscovery, error) {")
	require.NotContains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.NotContains(t, content, "func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {")
}

func TestZitadelAuthRouteAuthorizationFileOwnsRouteAndAllowlistPolicy(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_route_authorization.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func RouteRequiresZitadelAuth(route httproute.Descriptor) bool {")
	require.Contains(t, content, "func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {")
	require.Contains(t, content, "func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {")
	require.NotContains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.NotContains(t, content, "func (m *zitadelAuthMiddleware) verifyToken(r *http.Request, token string) (*zitadelIntrospectionResponse, error) {")
}
