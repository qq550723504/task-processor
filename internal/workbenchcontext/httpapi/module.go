package httpapi

import (
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

const ModuleName = "workbench-context"

type routeModule struct {
	handler *Handler
}

func NewModule(handler *Handler) kernelmodule.Module {
	return routeModule{handler: handler}
}

func (m routeModule) Name() string { return ModuleName }

func (m routeModule) Enabled(cfg *config.Config) bool {
	return m.handler != nil && cfg != nil && cfg.Workbench.Enabled
}

func (m routeModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(
		httproute.Descriptor{
			Method:                   http.MethodGet,
			Path:                     "/api/v1/workbench/context",
			Module:                   ModuleName,
			AuthPolicy:               httproute.AuthPolicyVerifiedIdentity,
			OrganizationAccessPolicy: httproute.OrganizationAccessPolicyContextRead,
			Handler:                  m.handler.GetContext,
		},
		httproute.Descriptor{
			Method:                     http.MethodPut,
			Path:                       "/api/v1/workbench/context/effective-organization",
			Module:                     ModuleName,
			AuthPolicy:                 httproute.AuthPolicyVerifiedIdentity,
			OrganizationAccessPolicy:   httproute.OrganizationAccessPolicyLiveSwitch,
			OrganizationTargetResolver: ResolveSwitchOrganizationTarget,
			Handler:                    m.handler.SwitchEffectiveOrganization,
		},
	)
	return nil
}
