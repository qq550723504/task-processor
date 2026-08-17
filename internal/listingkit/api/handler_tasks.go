package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GenerateListingKit(c *gin.Context) {
	if !h.requireSubscription(c, listingsubscription.ModuleStudio) {
		return
	}
	var req listingkit.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	// Source is internal lineage populated by the normalized source-facts bridge,
	// not a field callers may forge through the legacy public endpoint.
	req.Source = nil
	req.ImageURLs = absolutizeUploadedImageURLs(c, req.ImageURLs)
	// Task ownership remains the canonical request tenant so the caller can
	// later retrieve it through the normal tenant access scope. The subscription
	// guard may instead have admitted its legacy numeric fallback, which becomes
	// a separate billing identity for the worker's usage ledger.
	req.TenantID = requestTenantID(c, req.TenantID)
	req.BillingTenantID = subscriptionTenantID(c)
	if req.UserID == "" {
		req.UserID = requestUserID(c)
	}
	task, err := h.taskLifecycleService.CreateGenerateTask(requestContext(c, req.TenantID), &req)
	if err != nil {
		if writeStoreAccessError(c, err) {
			return
		}
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "task_creation_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task_id": task.ID, "tenant_id": task.TenantID, "status": task.Status, "created_at": task.CreatedAt})
}

func writeStoreAccessError(c *gin.Context, err error) bool {
	code := listingkit.StoreAccessErrorCode(err)
	if code == "" {
		return false
	}
	message := "Choose an enabled store available to your tenant and try again."
	if code == listingkit.StoreAccessDisabled {
		message = "Enable the selected store or choose another available store and try again."
	}
	c.JSON(http.StatusForbidden, gin.H{"error": code, "message": message})
	return true
}

func (h *handler) ListTasks(c *gin.Context) {
	var query listingkit.TaskListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	query.TenantID = requestTenantID(c, query.TenantID)
	page, err := h.taskLifecycleService.ListTasks(requestContext(c, query.TenantID), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *handler) GetTaskResult(c *gin.Context) {
	result, err := h.taskLifecycleService.GetTaskResult(requestContext(c), c.Param("task_id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, core.ErrTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "query_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
