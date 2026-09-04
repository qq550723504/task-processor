package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeSupportContractFileKeepsOnlySupportContract(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "type RuntimeSupportInput struct {")
	require.Contains(t, content, "type RuntimeSupport struct {")
	require.Contains(t, content, "func BuildRuntimeSupport(input RuntimeSupportInput) RuntimeSupport {")

	require.NotContains(t, content, "func buildRuntimeSupportRepositories() BuildServiceRepositories {")
	require.NotContains(t, content, "func buildRuntimeSupportHooks(cookieStore *sheinlogin.RedisStore) BuildServiceHooks {")
}

func TestRuntimeSupportRepositoriesFileOwnsRepositoryBundleBuilders(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_repositories.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func withApprovedAssetReader(")
	require.NotContains(t, content, "func buildRuntimeSupportRepositories()")
	require.NotContains(t, content, "CoreRepositoryBuilders")
	require.NotContains(t, content, "AdminRepositoryBuilders")
}

func TestRuntimeSupportHooksFileOwnsHookBundleBuilders(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime_support_hooks.go")
	require.NoError(t, err)
	content := string(src)

	require.Contains(t, content, "func buildRuntimeSupportHooks(cookieStore *sheinlogin.RedisStore) BuildServiceHooks {")
	require.Contains(t, content, "SheinCategoryResolverBuilder:")
	require.NotContains(t, content, "StudioImageGeneratorBuilder:")
}
