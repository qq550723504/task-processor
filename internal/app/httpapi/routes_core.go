package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/httproute"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	"task-processor/internal/taskrpcapi"
)

func buildCoreRouteDescriptors() []httproute.Descriptor {
	return []httproute.Descriptor{
		{
			Method: http.MethodGet,
			Path:   "/health",
			Module: "system",
			Handler: func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			},
		},
	}
}

func appendTaskRPCRouteDescriptors(routes []httproute.Descriptor, handler taskrpcapi.Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/health", Module: "management", Handler: handler.GetHealth},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/:task_id/status", Module: "management", Handler: handler.GetTaskStatus},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/retry", Module: "management", Handler: handler.RetryTask},
		httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/cancel", Module: "management", Handler: handler.CancelTask},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/queue-stats", Module: "management", Handler: handler.GetQueueStats},
	)
}

func appendSheinLoginRouteDescriptors(routes []httproute.Descriptor, handler sheinLoginRouteHandler) []httproute.Descriptor {
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

func appendSDSLoginRouteDescriptors(routes []httproute.Descriptor, handler sdslogin.HTTPRouteHandler) []httproute.Descriptor {
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

func appendSDSCatalogRouteDescriptors(routes []httproute.Descriptor, handlers ...sdshttpapi.HTTPRouteHandler) []httproute.Descriptor {
	var handler sdshttpapi.HTTPRouteHandler
	if len(handlers) > 0 {
		handler = handlers[0]
	}
	return sdshttpapi.AppendRouteDescriptors(routes, handler)
}
