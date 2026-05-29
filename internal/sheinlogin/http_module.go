package sheinlogin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
)

type HTTPRouteHandler interface {
	Health(c *gin.Context)
	ListAccounts(c *gin.Context)
	Login(c *gin.Context)
	Status(c *gin.Context)
	ListWarehouses(c *gin.Context)
	SubmitVerifyCode(c *gin.Context)
	CancelVerifyCodeWait(c *gin.Context)
	ClearCookie(c *gin.Context)
	GetLastFailure(c *gin.Context)
	ClearLastFailure(c *gin.Context)
}

const httpModuleName = "shein-login"

type httpModule struct {
	register func(reg *module.Registry) error
}

func NewHTTPModule(handler HTTPRouteHandler) module.Module {
	return httpModule{
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendRouteDescriptors(nil, handler)...)
			return nil
		},
	}
}

func (m httpModule) Name() string {
	return httpModuleName
}

func (httpModule) Enabled(*config.Config) bool {
	return true
}

func (m httpModule) Register(reg *module.Registry) error {
	if m.register != nil {
		return m.register(reg)
	}
	return nil
}

func AppendRouteDescriptors(routes []httproute.Descriptor, handler HTTPRouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/health", Module: "shein-login", Handler: handler.Health},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts", Module: "shein-login", Handler: handler.ListAccounts},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/login", Module: "shein-login", Handler: handler.Login},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/status", Module: "shein-login", Handler: handler.Status},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/warehouses", Module: "shein-login", Handler: handler.ListWarehouses},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/verify-code", Module: "shein-login", Handler: handler.SubmitVerifyCode},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/verify-code-wait", Module: "shein-login", Handler: handler.CancelVerifyCodeWait},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/cookie", Module: "shein-login", Handler: handler.ClearCookie},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: handler.GetLastFailure},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: handler.ClearLastFailure},
	)
}
