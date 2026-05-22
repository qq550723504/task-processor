package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func (h *handler) DispatchTaskGenerationNavigation(c *gin.Context) {
	var req listingkit.GenerationReviewNavigationDispatchRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	applyGenerationConditionalReadHeadersToNavigationTarget(c, &req)
	result, err := h.generationTaskService.DispatchTaskGenerationNavigation(requestContext(c), c.Param("task_id"), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, listingkit.ErrTaskNotFound) || errors.Is(err, listingkit.ErrGenerationActionNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "generation_navigation_dispatch_failed", "message": err.Error()})
		return
	}
	writeGenerationConditionalDispatchResponse(c, result.DeltaToken, result)
}
