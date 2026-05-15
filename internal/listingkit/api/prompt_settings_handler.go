package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/prompt"
)

func (h *handler) ListPromptSettings(c *gin.Context) {
	if h.tenantPromptStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	templates, err := h.tenantPromptStore.ListTenant(requestContext(c), requestTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prompt_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": templates})
}

func (h *handler) UpsertPromptSetting(c *gin.Context) {
	if h.tenantPromptStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	var req prompt.TenantPromptTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c, req.TenantID)
	if err := h.tenantPromptStore.Upsert(requestContext(c, req.TenantID), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt_save_failed", "message": err.Error()})
		return
	}
	saved, err := h.tenantPromptStore.GetEnabled(requestContext(c, req.TenantID), req.TenantID, req.Key)
	if err != nil {
		c.JSON(http.StatusOK, req)
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (h *handler) SetPromptSettingStatus(c *gin.Context) {
	if h.tenantPromptStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	tenantID := requestTenantID(c)
	key := c.Param("key")
	if err := h.tenantPromptStore.SetEnabled(requestContext(c, tenantID), tenantID, key, req.Enabled); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, prompt.ErrTenantPromptNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "prompt_status_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenant_id": tenantID, "key": key, "enabled": req.Enabled})
}
