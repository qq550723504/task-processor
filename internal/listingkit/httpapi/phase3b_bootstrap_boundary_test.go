package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractedBootstrapFilesOwnSplitAssemblers(t *testing.T) {
	t.Parallel()

	repositoriesSrc, err := os.ReadFile("bootstrap_repositories.go")
	require.NoError(t, err)
	require.Contains(t, string(repositoriesSrc), "func buildRepositories(")
	require.Contains(t, string(repositoriesSrc), "func buildCoreRepositories(")

	serviceConfigSrc, err := os.ReadFile("bootstrap_service_config.go")
	require.NoError(t, err)
	require.Contains(t, string(serviceConfigSrc), "func buildListingKitServiceConfig(")
	require.Contains(t, string(serviceConfigSrc), "func buildListingKitWorkflowDependencies(")

	runtimeSrc, err := os.ReadFile("bootstrap_runtime.go")
	require.NoError(t, err)
	require.Contains(t, string(runtimeSrc), "func buildServiceRuntime(")
	require.Contains(t, string(runtimeSrc), "func buildModuleRuntime(")
}
