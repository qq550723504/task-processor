package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ImportTaskHandler struct {
	repo ImportTaskRepository
}

type BatchCreateImportTaskRequest struct {
	StoreID        int64    `json:"storeId"`
	CategoryID     int64    `json:"categoryId"`
	Platform       string   `json:"platform"`
	TargetPlatform string   `json:"targetPlatform"`
	Region         string   `json:"region"`
	Priority       int      `json:"priority"`
	ProductIDs     []string `json:"productIds"`
	Remark         string   `json:"remark"`
}

type BatchCreateImportTaskResponse struct {
	CreatedCount int          `json:"createdCount"`
	Items        []ImportTask `json:"items"`
}

func NewImportTaskHandler(repo ImportTaskRepository) *ImportTaskHandler {
	return &ImportTaskHandler{repo: repo}
}

func (h *ImportTaskHandler) ListImportTasks(c *gin.Context) {
	query := ImportTaskQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestUserID(c),
		Page:        queryInt(c, "page", queryInt(c, "pageNo", 1)),
		PageSize:    queryInt(c, "page_size", queryInt(c, "pageSize", 20)),
		Platform:    strings.TrimSpace(c.Query("platform")),
		Region:      strings.TrimSpace(c.Query("region")),
		ProductID:   strings.TrimSpace(c.Query("productId")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.CategoryID = queryInt64Ptr(c, "categoryId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListImportTasks(withRequestUserID(c.Request.Context(), requestUserID(c)), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import_task_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ImportTaskHandler) BatchCreateImportTasks(c *gin.Context) {
	var req BatchCreateImportTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	tenantID := requestTenantID(c)
	productIDs := uniqueProductIDs(req.ProductIDs)
	if err := validateBatchCreateImportTask(tenantID, req, productIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_import_task", "message": err.Error()})
		return
	}

	storeID := req.StoreID
	categoryID := req.CategoryID
	tasks := make([]ImportTask, 0, len(productIDs))
	for _, productID := range productIDs {
		tasks = append(tasks, ImportTask{
			TenantID:       tenantID,
			StoreID:        &storeID,
			Platform:       strings.TrimSpace(req.Platform),
			TargetPlatform: strings.TrimSpace(req.TargetPlatform),
			SourcePlatform: strings.TrimSpace(req.Platform),
			Region:         strings.TrimSpace(req.Region),
			CategoryID:     &categoryID,
			ProductID:      productID,
			Status:         0,
			MaxRetryCount:  3,
			Remark:         strings.TrimSpace(req.Remark),
			Priority:       req.Priority,
		})
	}
	items, err := h.repo.BatchCreateImportTasks(withRequestUserID(c.Request.Context(), requestUserID(c)), tasks)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import_task_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, BatchCreateImportTaskResponse{CreatedCount: len(items), Items: items})
}

func (h *ImportTaskHandler) DeleteImportTask(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteImportTask(withRequestUserID(c.Request.Context(), requestUserID(c)), requestTenantID(c), id); err != nil {
		writeImportTaskError(c, err, "import_task_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func queryInt64Ptr(c *gin.Context, key string) *int64 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed := parseTenantID(value)
	if parsed <= 0 {
		return nil
	}
	return &parsed
}

func uniqueProductIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		productID := strings.TrimSpace(value)
		if productID == "" {
			continue
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		out = append(out, productID)
	}
	return out
}

func validateBatchCreateImportTask(tenantID int64, req BatchCreateImportTaskRequest, productIDs []string) error {
	switch {
	case tenantID <= 0:
		return errors.New("tenant id is required")
	case req.StoreID <= 0:
		return errors.New("storeId is required")
	case req.CategoryID <= 0:
		return errors.New("categoryId is required")
	case strings.TrimSpace(req.Platform) == "":
		return errors.New("platform is required")
	case len(productIDs) == 0:
		return errors.New("productIds is required")
	}
	return nil
}

func writeImportTaskError(c *gin.Context, err error, code string) {
	if errors.Is(err, ErrImportTaskNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "import_task_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": code, "message": err.Error()})
}
