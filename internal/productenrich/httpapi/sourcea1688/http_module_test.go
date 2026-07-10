package sourcea1688

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
	a1688 "task-processor/internal/product/sourcehandoff/a1688"
)

func TestHTTPModuleRegistersCreateListingKitTaskRoute(t *testing.T) {
	t.Parallel()

	reg := module.NewRegistry()
	err := NewHTTPModule(NewHandler(&moduleTaskCommandService{})).Register(reg)

	require.NoError(t, err)
	require.Equal(t, []string{"POST /api/v1/product-sourcing/1688/listingkit/tasks"}, moduleRouteKeys(reg.Routes()))
}

func TestBuildModuleReturnsHandlerAndModule(t *testing.T) {
	t.Parallel()

	result := BuildModule(&moduleTaskCommandService{})

	require.NotNil(t, result)
	require.NotNil(t, result.Handler)
	require.NotNil(t, result.Module)
	require.Equal(t, ModuleName, result.Module.Name())
	require.True(t, result.Module.Enabled(&config.Config{}))
}

func moduleRouteKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, route.Method+" "+route.Path)
	}
	return keys
}

type moduleTaskCommandService struct{}

func (moduleTaskCommandService) CreateTask(context.Context, a1688.CreateTaskCommand) (*a1688.CreateTaskResult, error) {
	return &a1688.CreateTaskResult{}, nil
}
