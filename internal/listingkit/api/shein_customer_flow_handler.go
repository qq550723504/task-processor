package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GetSheinSettings(c *gin.Context) {
	settings, err := h.settingsService.Get(requestContext(c), "shein", settingsNamespaceQuery{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *handler) UpdateSheinSettings(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	payload, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if !json.Valid(payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.settingsService.Update(requestContext(c), "shein", settingsNamespaceQuery{}, payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *handler) PreviewSheinPrice(c *gin.Context) {
	var req listingkit.SheinPricePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	preview, err := h.storeAdminService.PreviewSheinPrice(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) || errors.Is(err, listingkit.ErrTaskResultUnavailable) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "price_preview_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *handler) UpdateSheinFinalDraft(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.SheinFinalDraftUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	preview, err := h.storeAdminService.UpdateSheinFinalDraft(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) || errors.Is(err, listingkit.ErrTaskResultUnavailable) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "final_draft_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *handler) GetSubmissionEvents(c *gin.Context) {
	events, err := h.storeAdminService.GetSubmissionEvents(requestContext(c), c.Param("task_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) || errors.Is(err, listingkit.ErrTaskResultUnavailable) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "submission_events_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}
