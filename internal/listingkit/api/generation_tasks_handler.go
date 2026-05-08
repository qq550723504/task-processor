package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) GetTaskGenerationTasks(c *gin.Context) {
	var query listingkit.GenerationTaskQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	page, err := h.service.GetTaskGenerationTasks(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "generation_tasks_query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) GetTaskGenerationQueue(c *gin.Context) {
	var query listingkit.GenerationQueueQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if _, ok := c.GetQuery("retryable"); ok {
		query.RetryablePresent = true
	}
	if _, ok := c.GetQuery("render_preview_available"); ok {
		query.RenderPreviewAvailablePresent = true
	}
	applyGenerationConditionalReadHeaders(c, &query)
	page, err := h.service.GetTaskGenerationQueue(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "generation_queue_query_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalReadResponse(c, page.DeltaToken, page.NotModified, page)
}

func (h *handler) GetTaskGenerationReviewSession(c *gin.Context) {
	var query listingkit.GenerationQueueQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	applyGenerationConditionalReadHeaders(c, &query)
	result, err := h.service.GetTaskGenerationReviewSession(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "generation_review_session_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalReadResponse(c, result.DeltaToken, result.NotModified, result)
}

func (h *handler) GetTaskGenerationReviewPreview(c *gin.Context) {
	var query listingkit.GenerationQueueQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	applyGenerationConditionalReadHeaders(c, &query)
	result, err := h.service.GetTaskGenerationReviewPreview(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "generation_review_preview_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalReadResponse(c, result.DeltaToken, result.NotModified, result)
}

func (h *handler) RetryTaskGenerationTasks(c *gin.Context) {
	var req listingkit.RetryGenerationTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	page, err := h.service.RetryTaskGenerationTasks(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrGenerationTaskNotFound), errors.Is(err, listingkit.ErrGenerationTaskNotRetryable):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "generation_tasks_retry_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) ExecuteTaskGenerationAction(c *gin.Context) {
	var req listingkit.ExecuteGenerationActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.service.ExecuteTaskGenerationAction(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, listingkit.ErrTaskNotFound), errors.Is(err, listingkit.ErrGenerationActionNotFound):
			status = http.StatusNotFound
		case errors.Is(err, listingkit.ErrGenerationTaskNotFound), errors.Is(err, listingkit.ErrGenerationTaskNotRetryable):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "generation_action_execute_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalMutationResponse(c, result.DeltaToken, result)
}
