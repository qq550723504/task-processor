package httpapi

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

	require.Equal(t, "amazon-listing", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"POST /api/v1/amazon/listings/generate",
		"GET /api/v1/amazon/listings/tasks",
		"GET /api/v1/amazon/listings/tasks/:task_id",
		"GET /api/v1/amazon/listings/tasks/:task_id/workbench",
		"POST /api/v1/amazon/listings/tasks/:task_id/review",
		"POST /api/v1/amazon/listings/tasks/:task_id/submit",
	}, routeKeys(reg.Routes()))
}

type stubHandler struct{}

func (stubHandler) GenerateListing(*gin.Context)  {}
func (stubHandler) ListTaskQueue(*gin.Context)    {}
func (stubHandler) GetTaskResult(*gin.Context)    {}
func (stubHandler) GetTaskWorkbench(*gin.Context) {}
func (stubHandler) ReviewTask(*gin.Context)       {}
func (stubHandler) SubmitTask(*gin.Context)       {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
