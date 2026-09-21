package httpapi

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

const platformAdminModuleName = "listing-kit-platform-admin"

type platformAdminModule struct {
	handler               PlatformAdminRouteHandler
	subscriptionOwnerOnly bool
}

// NewPlatformAdminModule registers only the existing platform owner routes.
// It intentionally does not pull the product execution runtime into the
// account-center composition.
func NewPlatformAdminModule(handler PlatformAdminRouteHandler) kernelmodule.Module {
	return platformAdminModule{handler: handler}
}

// NewPlatformSubscriptionOwnerModule registers only routes backed by the
// subscription owner service. Directory and member-invitation routes require
// separate identity-provider dependencies and remain outside this composition.
func NewPlatformSubscriptionOwnerModule(handler PlatformAdminRouteHandler) kernelmodule.Module {
	return platformAdminModule{handler: handler, subscriptionOwnerOnly: true}
}

func (platformAdminModule) Name() string { return platformAdminModuleName }

func (platformAdminModule) Enabled(*config.Config) bool { return true }

func (m platformAdminModule) Register(reg *kernelmodule.Registry) error {
	if m.handler == nil {
		return nil
	}
	if m.subscriptionOwnerOnly {
		reg.AddRoutes(appendPlatformSubscriptionOwnerRouteDescriptors(nil, m.handler)...)
	} else {
		reg.AddRoutes(appendPlatformAdminRouteDescriptors(nil, m.handler)...)
	}
	return nil
}
