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

func TestNeutralRuntimeDelegatesProviderVerificationToReusableVerifier(t *testing.T) {
	t.Parallel()

	configSrc, err := os.ReadFile("../../authruntime/zitadel/config.go")
	require.NoError(t, err)
	require.Contains(t, string(configSrc), "func NewMiddleware(cfg Config, authzCfg AuthorizationConfig) gin.HandlerFunc {")

	middlewareSrc, err := os.ReadFile("../../authruntime/zitadel/middleware.go")
	require.NoError(t, err)
	middlewareContent := string(middlewareSrc)
	require.Contains(t, middlewareContent, "func (m *middleware) Handle(c *gin.Context) {")
	require.Contains(t, middlewareContent, "m.verifier.Verify(c.Request.Context(), token)")
	require.NotContains(t, middlewareContent, "func (m *middleware) verifyToken(")

	verifierSrc, err := os.ReadFile("../../authruntime/zitadel/verifier.go")
	require.NoError(t, err)
	verifierContent := string(verifierSrc)
	require.Contains(t, verifierContent, "type Verifier interface {")
	require.Contains(t, verifierContent, "func NewVerifier(cfg Config) Verifier {")
	require.Contains(t, verifierContent, "func (v *verifier) introspect(ctx context.Context, token string) (*IntrospectionResponse, error) {")
}

func TestLegacyListingKitProviderFilesAreAbsent(t *testing.T) {
	t.Parallel()

	_, middlewareErr := os.Stat("zitadel_auth_middleware.go")
	require.ErrorIs(t, middlewareErr, os.ErrNotExist)

	_, parsingErr := os.Stat("zitadel_auth_parsing_helpers.go")
	require.ErrorIs(t, parsingErr, os.ErrNotExist)
}
