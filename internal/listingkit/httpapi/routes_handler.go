package httpapi

type RouteHandler interface {
	TaskRouteHandler
	SettingsRouteHandler
	StoreRouteHandler
	SubscriptionRouteHandler
	PlatformAdminRouteHandler
	AdminRouteHandler
	StudioGenerationRouteHandler
	ZitadelSMSRouteHandler
	sheinSyncRouteHandler
	sheinPODImageLookupRouteHandler
}
