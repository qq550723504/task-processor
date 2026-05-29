package sheinlogin

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

	require.Equal(t, "shein-login", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"GET /api/v1/shein-login/health",
		"GET /api/v1/shein-login/accounts",
		"POST /api/v1/shein-login/accounts/:store_id/login",
		"GET /api/v1/shein-login/accounts/:store_id/status",
		"GET /api/v1/shein-login/accounts/:store_id/warehouses",
		"POST /api/v1/shein-login/accounts/:store_id/verify-code",
		"DELETE /api/v1/shein-login/accounts/:store_id/verify-code-wait",
		"DELETE /api/v1/shein-login/accounts/:store_id/cookie",
		"GET /api/v1/shein-login/accounts/:store_id/last-failure",
		"DELETE /api/v1/shein-login/accounts/:store_id/last-failure",
	}, routeKeys(reg.Routes()))
}

type stubHTTPRouteHandler struct{}

func (stubHTTPRouteHandler) Health(*gin.Context)               {}
func (stubHTTPRouteHandler) ListAccounts(*gin.Context)         {}
func (stubHTTPRouteHandler) Login(*gin.Context)                {}
func (stubHTTPRouteHandler) Status(*gin.Context)               {}
func (stubHTTPRouteHandler) ListWarehouses(*gin.Context)       {}
func (stubHTTPRouteHandler) SubmitVerifyCode(*gin.Context)     {}
func (stubHTTPRouteHandler) CancelVerifyCodeWait(*gin.Context) {}
func (stubHTTPRouteHandler) ClearCookie(*gin.Context)          {}
func (stubHTTPRouteHandler) GetLastFailure(*gin.Context)       {}
func (stubHTTPRouteHandler) ClearLastFailure(*gin.Context)     {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
