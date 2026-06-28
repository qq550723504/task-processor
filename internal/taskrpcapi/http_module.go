package taskrpcapi

import (
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
)

const httpModuleName = "task-rpc"

type httpModule struct {
	register func(reg *module.Registry) error
}

func NewHTTPModule(handler Handler) module.Module {
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

func AppendRouteDescriptors(routes []httproute.Descriptor, handler Handler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/health", Module: "management", Handler: handler.GetHealth},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/:task_id/status", Module: "management", Handler: handler.GetTaskStatus},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/management/tasks/queue-stats", Module: "management", Handler: handler.GetQueueStats},
	)
}
