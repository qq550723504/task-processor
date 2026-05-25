package listingadmin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type OperationStrategyHandler struct{ repo OperationStrategyRepository }

func NewOperationStrategyHandler(repo OperationStrategyRepository) *OperationStrategyHandler {
	return &OperationStrategyHandler{repo: repo}
}

func (h *OperationStrategyHandler) ListOperationStrategies(c *gin.Context) {
	pageNum, pageSize := requestPageParams(c)
	query := OperationStrategyQuery{
		TenantID:    requestTenantID(c),
		OwnerUserID: requestScopedOwnerUserID(c),
		Page:        pageNum,
		PageSize:    pageSize,
		Name:        strings.TrimSpace(c.Query("name")),
		Platform:    strings.TrimSpace(c.Query("platform")),
	}
	query.StoreID = queryInt64Ptr(c, "storeId")
	query.Status = queryInt16Ptr(c, "status")

	page, err := h.repo.ListOperationStrategies(requestIdentityContext(c), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation_strategy_list_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *OperationStrategyHandler) GetOperationStrategy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	strategy, err := h.repo.GetOperationStrategy(requestIdentityContext(c), requestTenantID(c), id)
	if err != nil {
		writeOperationStrategyError(c, err, "operation_strategy_get_failed")
		return
	}
	c.JSON(http.StatusOK, strategy)
}

func (h *OperationStrategyHandler) CreateOperationStrategy(c *gin.Context) {
	var req OperationStrategy
	if !bindJSON(c, &req) {
		return
	}
	req.TenantID = requestTenantID(c)
	if err := validateOperationStrategy(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_operation_strategy", "message": err.Error()})
		return
	}
	strategy, err := h.repo.CreateOperationStrategy(requestIdentityContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "operation_strategy_create_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, strategy)
}

func (h *OperationStrategyHandler) UpdateOperationStrategy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req OperationStrategy
	if !bindJSON(c, &req) {
		return
	}
	req.ID = id
	req.TenantID = requestTenantID(c)
	if err := validateOperationStrategy(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_operation_strategy", "message": err.Error()})
		return
	}
	strategy, err := h.repo.UpdateOperationStrategy(requestIdentityContext(c), &req)
	if err != nil {
		writeOperationStrategyError(c, err, "operation_strategy_update_failed")
		return
	}
	c.JSON(http.StatusOK, strategy)
}

func (h *OperationStrategyHandler) UpdateOperationStrategyStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int16  `json:"status"`
		Remark string `json:"remark"`
	}
	if !bindJSON(c, &req) {
		return
	}
	strategy, err := h.repo.UpdateOperationStrategyStatus(requestIdentityContext(c), requestTenantID(c), id, req.Status, req.Remark)
	if err != nil {
		writeOperationStrategyError(c, err, "operation_strategy_status_update_failed")
		return
	}
	c.JSON(http.StatusOK, strategy)
}

func (h *OperationStrategyHandler) DeleteOperationStrategy(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteOperationStrategy(requestIdentityContext(c), requestTenantID(c), id); err != nil {
		writeOperationStrategyError(c, err, "operation_strategy_delete_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func validateOperationStrategy(strategy *OperationStrategy) error {
	switch {
	case strategy.TenantID <= 0:
		return errors.New("tenant id is required")
	case strategy.StoreID <= 0:
		return errors.New("storeId is required")
	case strings.TrimSpace(strategy.Name) == "":
		return errors.New("name is required")
	case strings.TrimSpace(strategy.Platform) == "":
		return errors.New("platform is required")
	}
	return nil
}

func writeOperationStrategyError(c *gin.Context, err error, code string) {
	writeMappedHandlerError(c, err, code,
		handlerErrorRule{match: ErrOperationStrategyNotFound, status: http.StatusNotFound, errorCode: "operation_strategy_not_found"},
	)
}
