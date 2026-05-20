package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func buildCoreRouteDescriptors() []routeDescriptor {
	return []routeDescriptor{
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

func appendTaskRPCRouteDescriptors(routes []routeDescriptor, handler taskRPCRouteHandler) []routeDescriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/health", Module: "management", Handler: handler.GetHealth},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/:task_id/status", Module: "management", Handler: handler.GetTaskStatus},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/retry", Module: "management", Handler: handler.RetryTask},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/management/tasks/:task_id/cancel", Module: "management", Handler: handler.CancelTask},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/queue-stats", Module: "management", Handler: handler.GetQueueStats},
	)
}

func appendSheinLoginRouteDescriptors(routes []routeDescriptor, handler sheinLoginRouteHandler) []routeDescriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/health", Module: "shein-login", Handler: handler.Health},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts", Module: "shein-login", Handler: handler.ListAccounts},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/login", Module: "shein-login", Handler: handler.Login},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/status", Module: "shein-login", Handler: handler.Status},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/warehouses", Module: "shein-login", Handler: handler.ListWarehouses},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/shein-login/accounts/:store_id/verify-code", Module: "shein-login", Handler: handler.SubmitVerifyCode},
		routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/verify-code-wait", Module: "shein-login", Handler: handler.CancelVerifyCodeWait},
		routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/cookie", Module: "shein-login", Handler: handler.ClearCookie},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: handler.GetLastFailure},
		routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/shein-login/accounts/:store_id/last-failure", Module: "shein-login", Handler: handler.ClearLastFailure},
	)
}

func appendSDSLoginRouteDescriptors(routes []routeDescriptor, handler sdsLoginRouteHandler) []routeDescriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/health", Module: "sds-login", Handler: handler.Health},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/status", Module: "sds-login", Handler: handler.Status},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/login", Module: "sds-login", Handler: handler.Login},
		routeDescriptor{Method: http.MethodPost, Path: "/api/v1/sds-login/manual-login", Module: "sds-login", Handler: handler.ManualLogin},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds-login/auth-state", Module: "sds-login", Handler: handler.GetAuthState},
		routeDescriptor{Method: http.MethodDelete, Path: "/api/v1/sds-login/state", Module: "sds-login", Handler: handler.ClearState},
	)
}

func appendSDSCatalogRouteDescriptors(routes []routeDescriptor, handlers ...sdsCatalogRouteHandler) []routeDescriptor {
	var handler sdsCatalogRouteHandler
	if len(handlers) > 0 {
		handler = handlers[0]
	}
	if handler == nil {
		return routes
	}
	return append(routes,
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/products", Module: "sds", Handler: handler.ListSDSProducts},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/products/:product_id", Module: "sds", Handler: handler.GetSDSProduct},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/categories", Module: "sds", Handler: handler.ListSDSCategories},
		routeDescriptor{Method: http.MethodGet, Path: "/api/v1/sds/shipment-areas", Module: "sds", Handler: handler.ListSDSShipmentAreas},
	)
}
