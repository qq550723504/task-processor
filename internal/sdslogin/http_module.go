package sdslogin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
)

type HTTPRouteHandler interface {
	Health(c *gin.Context)
	Status(c *gin.Context)
	Login(c *gin.Context)
	ManualLogin(c *gin.Context)
	GetAuthState(c *gin.Context)
	ClearState(c *gin.Context)
}

const httpModuleName = "sds-login"

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
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/health", Module: "sds-login", Handler: handler.Health},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/status", Module: "sds-login", Handler: handler.Status},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/login", Module: "sds-login", Handler: handler.Login},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/manual-login", Module: "sds-login", Handler: handler.ManualLogin},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/auth-state", Module: "sds-login", Handler: handler.GetAuthState},
		httproute.Descriptor{Method: http.MethodDelete, Path: "/api/v1/sds-login/state", Module: "sds-login", Handler: handler.ClearState},
	)
}
