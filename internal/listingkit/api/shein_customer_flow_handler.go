package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GetSheinSettings(c *gin.Context) {
	settings, err := h.service.GetSheinSettings(requestContext(c))
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
	var req listingkit.SheinSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	settings, err := h.service.UpdateSheinSettings(requestContext(c), &req)
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
	preview, err := h.service.PreviewSheinPrice(requestContext(c), c.Param("task_id"), &req)
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
	preview, err := h.service.UpdateSheinFinalDraft(requestContext(c), c.Param("task_id"), &req)
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
	events, err := h.service.GetSubmissionEvents(requestContext(c), c.Param("task_id"))
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
