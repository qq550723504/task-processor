package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) GetSettingsNamespace(c *gin.Context) {
	settings, err := h.settingsService.Get(requestContext(c), c.Param("namespace"), settingsNamespaceQuery{
		TenantID:   requestTenantID(c),
		Scope:      c.Query("scope"),
		ClientName: c.Query("client_name"),
	})
	if err != nil {
		if errors.Is(err, errUnsupportedSettingsNamespace) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "settings_namespace_not_found",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "settings_read_failed",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *handler) GetSettingsHealth(c *gin.Context) {
	if h.settingsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "settings_service_unavailable",
			"message": "ListingKit settings service is not available",
		})
		return
	}
	health, err := h.settingsService.Health(requestContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "settings_health_failed",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, health)
}

func (h *handler) ListSettingsNamespaces(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"items": h.settingsService.ListSchemas(),
	})
}

func (h *handler) GetSettingsNamespaceSchema(c *gin.Context) {
	schema, err := h.settingsService.GetSchema(c.Param("namespace"))
	if err != nil {
		if errors.Is(err, errUnsupportedSettingsNamespace) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "settings_namespace_not_found",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "settings_schema_failed",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, schema)
}

func (h *handler) UpdateSettingsNamespace(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "shein" && !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}
	if !json.Valid(payload) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "invalid JSON payload",
		})
		return
	}
	settings, err := h.settingsService.Update(requestContext(c), namespace, settingsNamespaceQuery{
		TenantID:   requestTenantID(c),
		Scope:      c.Query("scope"),
		ClientName: c.Query("client_name"),
	}, payload)
	if err != nil {
		if errors.Is(err, errUnsupportedSettingsNamespace) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "settings_namespace_not_found",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "settings_update_failed",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, settings)
}
