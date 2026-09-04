package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractedBootstrapFilesOwnSplitAssemblers(t *testing.T) {
	t.Parallel()

	repositoryContractsSrc, err := os.ReadFile("bootstrap_repositories_contracts.go")
	require.NoError(t, err)
	require.Contains(t, string(repositoryContractsSrc), "type builtRepositories struct")
	require.NotContains(t, string(repositoryContractsSrc), "type repositoryAssembly struct")

	_, err = os.Stat("bootstrap_repositories_core.go")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat("bootstrap_repositories_admin.go")
	require.ErrorIs(t, err, os.ErrNotExist)

	repositoryMergeSrc, err := os.ReadFile("bootstrap_repositories_merge.go")
	require.NoError(t, err)
	require.Contains(t, string(repositoryMergeSrc), "func buildRepositories(")
	require.NotContains(t, string(repositoryMergeSrc), "func mergeBuiltRepositories(")

	contractsSrc, err := os.ReadFile("bootstrap_contracts.go")
	require.NoError(t, err)
	require.Contains(t, string(contractsSrc), "type BuildServiceInput struct")
	require.Contains(t, string(contractsSrc), "type BuildServiceHooks struct")

	validationSrc, err := os.ReadFile("bootstrap_validation.go")
	require.NoError(t, err)
	require.Contains(t, string(validationSrc), "func (in BuildServiceInput) Validate() error")

	closersSrc, err := os.ReadFile("bootstrap_closers.go")
	require.NoError(t, err)
	require.Contains(t, string(closersSrc), "type closerStack struct")
	require.NotContains(t, string(closersSrc), "func buildNamedWithClosers")

	moduleServiceSrc, err := os.ReadFile("bootstrap_module_service.go")
	require.NoError(t, err)
	require.Contains(t, string(moduleServiceSrc), "func buildModuleService(")
	require.Contains(t, string(moduleServiceSrc), "func assembleServiceBundle(")

	serviceConfigSrc, err := os.ReadFile("bootstrap_service_config.go")
	require.NoError(t, err)
	require.Contains(t, string(serviceConfigSrc), "func buildListingKitServiceConfig(")
	require.Contains(t, string(serviceConfigSrc), "func buildListingKitWorkflowDependencies(")

	runtimeSrc, err := os.ReadFile("bootstrap_runtime.go")
	require.NoError(t, err)
	require.Contains(t, string(runtimeSrc), "func buildServiceRuntime(")
	require.Contains(t, string(runtimeSrc), "func buildModuleRuntime(")
}
