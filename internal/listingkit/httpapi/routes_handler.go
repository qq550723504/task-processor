package httpapi

type RouteHandler interface {
	TaskRouteHandler
	SettingsRouteHandler
	StoreRouteHandler
	SubscriptionRouteHandler
	PlatformAdminRouteHandler
	AdminRouteHandler
	StudioGenerationRouteHandler
	sheinSyncRouteHandler
	sheinPODImageLookupRouteHandler
}
