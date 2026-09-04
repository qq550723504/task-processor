package httpapi

type RouteHandler interface {
	TaskRouteHandler
	SettingsRouteHandler
	StoreRouteHandler
	SubscriptionRouteHandler
	PlatformAdminRouteHandler
	AdminRouteHandler
	ZitadelSMSRouteHandler
	sheinSyncRouteHandler
	sheinPODImageLookupRouteHandler
}
