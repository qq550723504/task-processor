package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZitadelAuthFileUsesNeutralRuntimeAliasesAndAdapterState(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, `"task-processor/internal/authruntime/zitadel"`)
	require.Contains(t, content, "type zitadelAuthConfig = zitadelruntime.Config")
	require.Contains(t, content, "type zitadelAuthorizationConfig = zitadelruntime.AuthorizationConfig")
	require.Contains(t, content, "type listingKitZitadelRuntimeConfig struct {")
	require.Contains(t, content, "Authorizer  *authz.ListingKitAuthorizer")
	require.NotContains(t, content, "type zitadelDiscovery struct {")
	require.NotContains(t, content, "type zitadelIntrospectionResponse struct {")
	require.NotContains(t, content, "type zitadelAuthMiddleware struct {")
}

func TestZitadelAuthRuntimeFileOwnsRuntimeConfigAndMiddlewareFactory(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_runtime.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.Contains(t, content, "func ConfigureListingKitAuthorization(platformAdminUsers []string, platformAdminRoles []string) error {")
	require.Contains(t, content, "func NewZitadelAuthMiddlewareFromEnv() gin.HandlerFunc {")
	require.Contains(t, content, "return zitadelruntime.NewMiddleware(runtimeCfg.AuthConfig, runtimeCfg.AuthzConfig)")
	require.NotContains(t, content, "func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {")
	require.NotContains(t, content, "func (m *zitadelAuthMiddleware) Handle(c *gin.Context) {")
}

func TestZitadelAuthRouteAuthorizationFileOwnsRouteAndPermissionMapping(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("zitadel_auth_route_authorization.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func RouteRequiresZitadelAuth(route httproute.Descriptor) bool {")
	require.Contains(t, content, "func NewRouteRoleMiddleware(route httproute.Descriptor) gin.HandlerFunc {")
	require.NotContains(t, content, "func ConfigureListingKitZitadelAuth(cfg config.ListingKitZitadelConfig) {")
	require.NotContains(t, content, "func (m *zitadelAuthMiddleware) verifyToken(r *http.Request, token string) (*zitadelIntrospectionResponse, error) {")
	require.NotContains(t, content, "func authorizeZitadelIdentity(identity *zitadelIntrospectionResponse, cfg zitadelAuthorizationConfig) (bool, string) {")
	require.NotContains(t, content, "type zitadelDiscovery struct {")
}

func TestNeutralRuntimeMiddlewareOwnsProviderVerification(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../../authruntime/zitadel/middleware.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func NewMiddleware(cfg Config, authzCfg AuthorizationConfig) gin.HandlerFunc {")
	require.Contains(t, content, "func (m *middleware) Handle(c *gin.Context) {")
	require.Contains(t, content, "func (m *middleware) verifyToken(r *http.Request, token string) (*IntrospectionResponse, error) {")
}

func TestLegacyListingKitProviderFilesAreAbsent(t *testing.T) {
	t.Parallel()

	_, middlewareErr := os.Stat("zitadel_auth_middleware.go")
	require.ErrorIs(t, middlewareErr, os.ErrNotExist)

	_, parsingErr := os.Stat("zitadel_auth_parsing_helpers.go")
	require.ErrorIs(t, parsingErr, os.ErrNotExist)
}
