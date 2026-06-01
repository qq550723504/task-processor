package sdslogin

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
	module := NewHTTPModule(stubHTTPRouteHandler{})

	require.Equal(t, "sds-login", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"GET /api/v1/sds-login/health",
		"GET /api/v1/sds-login/status",
		"POST /api/v1/sds-login/login",
		"POST /api/v1/sds-login/manual-login",
		"GET /api/v1/sds-login/auth-state",
		"DELETE /api/v1/sds-login/state",
	}, routeKeys(reg.Routes()))
}

type stubHTTPRouteHandler struct{}

func (stubHTTPRouteHandler) Health(*gin.Context)       {}
func (stubHTTPRouteHandler) Status(*gin.Context)       {}
func (stubHTTPRouteHandler) Login(*gin.Context)        {}
func (stubHTTPRouteHandler) ManualLogin(*gin.Context)  {}
func (stubHTTPRouteHandler) GetAuthState(*gin.Context) {}
func (stubHTTPRouteHandler) ClearState(*gin.Context)   {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
