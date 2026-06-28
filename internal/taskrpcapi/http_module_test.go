package taskrpcapi

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestNewHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubHandler{})

	require.Equal(t, "task-rpc", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"GET /api/v1/management/tasks/health",
		"GET /api/v1/management/tasks/:task_id/status",
		"GET /api/v1/management/tasks/queue-stats",
	}, routeKeys(reg.Routes()))
}

type stubHandler struct{}

func (stubHandler) GetTaskStatus(*gin.Context) {}
func (stubHandler) GetQueueStats(*gin.Context) {}
func (stubHandler) GetHealth(*gin.Context)     {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
