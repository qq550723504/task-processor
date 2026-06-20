package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (h *handler) CreateStudioBatchRun(c *gin.Context) {
	if h == nil || h.studioBatchRunService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "studio_batch_run_unavailable", "message": "studio batch run service is not configured"})
		return
	}

	var req listingkit.CreateStudioBatchRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	run, items, err := h.studioBatchRunService.CreateStudioBatchRun(requestContext(c), &req)
	if err != nil {
		status := classifyCreateStudioBatchRunError(err)
		c.JSON(status, gin.H{"error": "studio_batch_run_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run": run, "items": items})
}

func (h *handler) GetStudioBatchRun(c *gin.Context) {
	if h == nil || h.studioBatchRunService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "studio_batch_run_unavailable", "message": "studio batch run service is not configured"})
		return
	}

	run, err := h.studioBatchRunService.GetStudioBatchRun(requestContext(c), c.Param("run_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_run_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *handler) ListStudioBatchRunItems(c *gin.Context) {
	if h == nil || h.studioBatchRunService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "studio_batch_run_unavailable", "message": "studio batch run service is not configured"})
		return
	}

	items, err := h.studioBatchRunService.ListStudioBatchRunItems(requestContext(c), c.Param("run_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_run_items_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *handler) CancelStudioBatchRun(c *gin.Context) {
	if h == nil || h.studioBatchRunService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "studio_batch_run_unavailable", "message": "studio batch run service is not configured"})
		return
	}

	if err := h.studioBatchRunService.CancelStudioBatchRun(requestContext(c), c.Param("run_id")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_run_cancel_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func (h *handler) RecoverStudioBatchRun(c *gin.Context) {
	if h == nil || h.studioBatchRunService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "studio_batch_run_unavailable", "message": "studio batch run service is not configured"})
		return
	}

	if err := h.studioBatchRunService.RecoverStudioBatchRun(requestContext(c), c.Param("run_id")); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrStudioBatchActionValidation):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "studio_batch_run_recover_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

func classifyCreateStudioBatchRunError(err error) int {
	switch {
	case err == nil:
		return http.StatusAccepted
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, listingkit.ErrStudioSessionNotFound):
		return http.StatusNotFound
	case isStudioBatchRunValidationError(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func isStudioBatchRunValidationError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "batch_ids is required") || strings.Contains(message, "duplicate batch_id")
}
