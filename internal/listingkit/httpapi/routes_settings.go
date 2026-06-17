package httpapi

import "github.com/gin-gonic/gin"

type SettingsStoreRouteHandler interface {
	ListSheinStoreProfiles(c *gin.Context)
	UpsertSheinStoreProfile(c *gin.Context)
	DeleteSheinStoreProfile(c *gin.Context)
	GetSheinSettings(c *gin.Context)
	UpdateSheinSettings(c *gin.Context)
	GetAIClientSettings(c *gin.Context)
	UpdateAIClientSettings(c *gin.Context)
}

type SettingsRouteHandler interface {
	ListSettingsNamespaces(c *gin.Context)
	GetSettingsHealth(c *gin.Context)
	GetSettingsNamespaceSchema(c *gin.Context)
	GetSettingsNamespace(c *gin.Context)
	UpdateSettingsNamespace(c *gin.Context)
	SettingsStoreRouteHandler
}
