package httpapi

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestNewHTTPModuleRegistersProductAndImageRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubProductHandler{}, stubImageHandler{})

	require.Equal(t, "product", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"POST /api/v1/products/generate",
		"GET /api/v1/products/tasks/:task_id",
		"POST /api/v1/images/process",
		"GET /api/v1/images/tasks/:task_id",
		"POST /api/v1/images/tasks/:task_id/review",
	}, routeKeys(reg.Routes()))
}

type stubProductHandler struct{}

func (stubProductHandler) GenerateProduct(*gin.Context) {}
func (stubProductHandler) GetTaskResult(*gin.Context)   {}

type stubImageHandler struct{}

func (stubImageHandler) ProcessImages(*gin.Context) {}
func (stubImageHandler) GetTaskResult(*gin.Context) {}
func (stubImageHandler) ReviewTask(*gin.Context)    {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
