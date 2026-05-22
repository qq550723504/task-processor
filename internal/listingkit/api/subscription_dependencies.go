package api

import "task-processor/internal/listingsubscription"

func withSubscriptionDependencies(apply func(*subscriptionDependencies)) HandlerOption {
	return withHandlerState(func(h *handler) {
		apply(&h.subscriptionDependencies)
	})
}

func withSubscriptionDependency[Dep comparable](dep Dep, apply func(Dep, *subscriptionDependencies)) HandlerOption {
	return withSubscriptionDependencies(func(subscription *subscriptionDependencies) {
		var zero Dep
		if dep == zero {
			return
		}
		apply(dep, subscription)
	})
}

func withSubscriptionConfig(deps SubscriptionDependencies) HandlerOption {
	options := []HandlerOption{
		WithPlatformSubscriptionAccess(deps.PlatformAdminUsers, deps.PlatformAdminRoles),
		WithSubscriptionService(deps.Service),
	}
	return func(h *handler) {
		for _, option := range options {
			if option != nil {
				option(h)
			}
		}
	}
}

func WithSubscriptionService(service *listingsubscription.Service) HandlerOption {
	return withSubscriptionDependency(service, func(service *listingsubscription.Service, subscription *subscriptionDependencies) {
		subscription.subscriptionService = service
		subscription.subscriptionHandler = listingsubscription.NewHandler(service)
	})
}

func WithPlatformSubscriptionAccess(users []string, roles []string) HandlerOption {
	return withSubscriptionDependencies(func(subscription *subscriptionDependencies) {
		subscription.platformAdminUsers = append([]string(nil), users...)
		subscription.platformAdminRoles = append([]string(nil), roles...)
	})
}
