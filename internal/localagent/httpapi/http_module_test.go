package httpapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
	"task-processor/internal/localagent"
)

func TestHTTPModuleRegistersLocalAgentRoutes(t *testing.T) {
	result := BuildModule(localagent.NewService(nil))
	require.NotNil(t, result)
	require.Equal(t, ModuleName, result.Module.Name())
	require.True(t, result.Module.Enabled(&config.Config{}))

	registry := module.NewRegistry()
	require.NoError(t, result.Module.Register(registry))
	require.Len(t, registry.Routes(), 4)
}
