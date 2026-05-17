package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
