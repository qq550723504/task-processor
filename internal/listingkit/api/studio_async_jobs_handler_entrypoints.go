package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) StartStudioAsyncJob(c *gin.Context) {
	var req startStudioAsyncJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path != "/studio/designs" && req.Path != "/studio/product-images" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "unsupported async job path"})
		return
	}
	if len(req.Body) == 0 {
		req.Body = json.RawMessage(`{}`)
	}
	metric := "design_jobs"
	if req.Path == "/studio/product-images" {
		metric = "product_image_jobs"
	}
	if !h.authorizeSubscriptionUsage(c, listingsubscription.ModuleStudio, metric, 1) {
		return
	}

	reqCtx := requestContext(c)
	job, err := h.studioAsyncJobs.create(reqCtx, req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "async_job_create_failed", "message": err.Error()})
		return
	}
	studioAsyncJobLogger.WithFields(studioAsyncLogFields(reqCtx, logrus.Fields{
		"job_id":       job.ID,
		"path":         req.Path,
		"session_id":   strings.TrimSpace(req.SessionID),
		"body_bytes":   len(req.Body),
		"usage_metric": metric,
	})).Info("studio async job accepted")
	ctx := detachedRequestContext(c)
	baseURL := requestBaseURL(c)
	sessionID := strings.TrimSpace(req.SessionID)
	if req.Path == "/studio/designs" {
		h.syncStudioDesignAsyncJobSession(reqCtx, sessionID, listingkit.StudioAsyncJobStatusRunning, job.ID, "")
	}
	go h.runStudioAsyncJob(ctx, job.ID, req.Path, req.Body, sessionID, baseURL, metric)

	c.JSON(http.StatusAccepted, job)
}

func (h *handler) GetStudioAsyncJob(c *gin.Context) {
	job, ok := h.studioAsyncJobs.get(requestContext(c), c.Param("job_id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "studio async job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}
