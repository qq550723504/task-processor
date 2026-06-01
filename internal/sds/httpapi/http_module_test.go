package httpapi

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestNewHTTPModuleRegistersRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubHTTPRouteHandler{})

	require.Equal(t, "sds", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"GET /api/v1/sds/products",
		"GET /api/v1/sds/products/:product_id",
		"GET /api/v1/sds/categories",
		"GET /api/v1/sds/shipment-areas",
	}, routeKeys(reg.Routes()))
}

func TestBuildModuleReturnsHandlerAndModule(t *testing.T) {
	t.Parallel()

	result := BuildModule(logrus.New(), &config.Config{})
	require.NotNil(t, result)
	require.NotNil(t, result.Handler)
	require.NotNil(t, result.Module)
}

type stubHTTPRouteHandler struct{}

func (stubHTTPRouteHandler) ListSDSProducts(*gin.Context)      {}
func (stubHTTPRouteHandler) GetSDSProduct(*gin.Context)        {}
func (stubHTTPRouteHandler) ListSDSCategories(*gin.Context)    {}
func (stubHTTPRouteHandler) ListSDSShipmentAreas(*gin.Context) {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
