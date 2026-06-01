package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type studioSessionHandler struct {
	service listingkit.StudioSessionHandlerService
}

func NewStudioSessionHandler(service listingkit.StudioSessionHandlerService) (listingkit.StudioSessionHandler, error) {
	if service == nil {
		return nil, errors.New("service cannot be nil")
	}
	return &studioSessionHandler{service: service}, nil
}

func (h *studioSessionHandler) EnsureStudioSession(c *gin.Context) {
	var req listingkit.EnsureStudioSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if req.UserID == "" {
		req.UserID = requestUserID(c)
	}
	detail, err := h.service.EnsureStudioSession(requestContext(c), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "studio_session_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) GetStudioSession(c *gin.Context) {
	detail, err := h.service.GetStudioSession(requestContext(c), c.Param("session_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_session_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) UpdateStudioSession(c *gin.Context) {
	var req listingkit.UpdateStudioSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.UpdateStudioSession(requestContext(c), c.Param("session_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, listingkit.ErrStudioSessionConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "studio_session_update_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) ReplaceStudioSessionDesigns(c *gin.Context) {
	var req listingkit.ReplaceStudioSessionDesignsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.ReplaceStudioSessionDesigns(requestContext(c), c.Param("session_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, listingkit.ErrStudioSessionConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "studio_session_designs_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) ListStudioSessionGallery(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "240"))
	response, err := h.service.ListStudioSessionGallery(requestContext(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "studio_session_gallery_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *studioSessionHandler) ListStudioBatches(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	response, err := h.service.ListStudioBatches(requestContext(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "studio_batch_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *studioSessionHandler) GetStudioBatch(c *gin.Context) {
	detail, err := h.service.GetStudioBatch(requestContext(c), c.Param("batch_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) StartStudioBatchGeneration(c *gin.Context) {
	detail, err := h.service.StartStudioBatchGeneration(requestContext(c), c.Param("batch_id"))
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_generate_failed", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) RetryStudioBatchItems(c *gin.Context) {
	var req listingkit.RetryStudioBatchItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.RetryStudioBatchItems(requestContext(c), c.Param("batch_id"), &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_retry_failed", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) ApproveStudioBatchDesigns(c *gin.Context) {
	var req listingkit.ApproveStudioBatchDesignsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.ApproveStudioBatchDesigns(requestContext(c), c.Param("batch_id"), &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_approve_failed", err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) CreateStudioBatchTasks(c *gin.Context) {
	var req listingkit.CreateStudioBatchTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.service.CreateStudioBatchTasks(requestContext(c), c.Param("batch_id"), &req)
	if err != nil {
		writeStudioBatchActionError(c, "studio_batch_tasks_failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *studioSessionHandler) AppendStudioSessionDesigns(c *gin.Context) {
	var req listingkit.AppendStudioSessionDesignsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.AppendStudioSessionDesigns(requestContext(c), c.Param("session_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, listingkit.ErrStudioSessionConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "studio_session_designs_append_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) UpsertStudioBatch(c *gin.Context) {
	var req listingkit.UpsertStudioBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	detail, err := h.service.UpsertStudioBatch(requestContext(c), &req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, listingkit.ErrStudioSessionConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "studio_batch_save_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *studioSessionHandler) DeleteStudioBatch(c *gin.Context) {
	if err := h.service.DeleteStudioBatch(requestContext(c), c.Param("batch_id")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrStudioSessionNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "studio_batch_delete_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeStudioBatchActionError(c *gin.Context, errorCode string, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, listingkit.ErrStudioSessionNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		status = http.StatusNotFound
	case errors.Is(err, listingkit.ErrStudioSessionConflict):
		status = http.StatusConflict
	case isStudioBatchActionValidationError(err):
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"error": errorCode, "message": err.Error()})
}

func isStudioBatchActionValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "item_ids is required") ||
		strings.Contains(message, "design_ids is required") ||
		strings.Contains(message, "is not approved")
}
