package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/promptmgmt"
)

type Handler struct {
	service promptmgmt.Service
}

func NewHandler(service promptmgmt.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListPromptTemplateCatalog(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": h.service.ListTemplateCatalog()})
}

func (h *Handler) GetPromptTemplateSchema(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	schema, err := h.service.GetTemplateSchema(c.Param("key"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, promptmgmt.ErrTemplateNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "prompt_schema_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schema)
}

func (h *Handler) ListPromptTemplates(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	templates, err := h.service.ListTenantTemplates(requestContext(c), requestTenantID(c))
	if err != nil {
		if errors.Is(err, promptmgmt.ErrServiceUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prompt_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": templates})
}

func (h *Handler) UpsertPromptTemplate(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": "tenant prompt store is not configured"})
		return
	}
	var req promptmgmt.UpsertTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.TenantID = requestTenantID(c, req.TenantID)
	saved, err := h.service.UpsertTenantTemplate(requestContext(c, req.TenantID), req)
	if errors.Is(err, promptmgmt.ErrServiceUnavailable) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": err.Error()})
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, promptmgmt.ErrTemplateNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "prompt_save_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, saved)
}

func (h *Handler) SetPromptTemplateStatus(c *gin.Context) {
	if h.service == nil {
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
	statusPayload, err := h.service.SetTenantTemplateStatus(requestContext(c, tenantID), tenantID, key, req.Enabled)
	if errors.Is(err, promptmgmt.ErrServiceUnavailable) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt_store_unavailable", "message": err.Error()})
		return
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, promptmgmt.ErrTemplateNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "prompt_status_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statusPayload)
}

func requestContext(c *gin.Context, candidates ...string) context.Context {
	return listingkit.WithTenantID(c.Request.Context(), requestTenantID(c, candidates...))
}

func requestTenantID(c *gin.Context, candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	for _, header := range []string{"X-Tenant-ID", "X-Tenant-Id", "X-Tenant", "tenant-id"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(c.Query("tenant_id")); value != "" {
		return value
	}
	return listingkit.DefaultTenantID
}
