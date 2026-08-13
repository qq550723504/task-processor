package api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
)

func (h *handler) GetTaskGenerationTasks(c *gin.Context) {
	var query listingkit.GenerationTaskQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	page, err := h.generationTaskService.GetTaskGenerationTasks(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrTaskNotFound) {
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
	page, err := h.generationTaskService.GetTaskGenerationQueue(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrTaskNotFound) {
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
	result, err := h.generationTaskService.GetTaskGenerationReviewSession(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrTaskNotFound) {
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
	result, err := h.generationTaskService.GetTaskGenerationReviewPreview(requestContext(c), c.Param("task_id"), &query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrTaskNotFound) {
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
	page, err := h.generationTaskService.RetryTaskGenerationTasks(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, core.ErrTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, core.ErrGenerationTaskNotFound), errors.Is(err, core.ErrGenerationTaskNotRetryable):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "generation_tasks_retry_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) RetryTaskChildTask(c *gin.Context) {
	if h.childTaskRetryService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "child_task_retry_not_supported", "message": "child task retry is not supported"})
		return
	}
	var req listingkit.RetryChildTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	if strings.TrimSpace(req.Kind) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "kind is required"})
		return
	}
	result, err := h.childTaskRetryService.ScheduleTaskChildRetry(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, core.ErrTaskNotFound), errors.Is(err, listingkit.ErrTaskResultUnavailable), errors.Is(err, core.ErrChildTaskNotFound):
			status = http.StatusNotFound
		case errors.Is(err, core.ErrChildTaskRetryInvalidRequest):
			status = http.StatusBadRequest
		case errors.Is(err, core.ErrChildTaskNotRetryable), errors.Is(err, core.ErrChildTaskRetryConflict), errors.Is(err, listingkit.ErrSDSRepairRetryInProgress):
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": "child_task_retry_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h *handler) ExecuteTaskGenerationAction(c *gin.Context) {
	var req listingkit.ExecuteGenerationActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	result, err := h.generationTaskService.ExecuteTaskGenerationAction(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, core.ErrTaskNotFound), errors.Is(err, core.ErrGenerationActionNotFound):
			status = http.StatusNotFound
		case errors.Is(err, core.ErrGenerationTaskNotFound), errors.Is(err, core.ErrGenerationTaskNotRetryable):
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "generation_action_execute_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalMutationResponse(c, result.DeltaToken, result)
}

