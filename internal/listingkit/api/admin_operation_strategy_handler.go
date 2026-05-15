package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) ListAdminOperationStrategies(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.ListOperationStrategies(c)
}

func (h *handler) GetAdminOperationStrategy(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.GetOperationStrategy(c)
}

func (h *handler) CreateAdminOperationStrategy(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.CreateOperationStrategy(c)
}

func (h *handler) UpdateAdminOperationStrategy(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.UpdateOperationStrategy(c)
}

func (h *handler) UpdateAdminOperationStrategyStatus(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.UpdateOperationStrategyStatus(c)
}

func (h *handler) DeleteAdminOperationStrategy(c *gin.Context) {
	if !h.requireOperationStrategyHandler(c) {
		return
	}
	h.operationStrategyHandler.DeleteOperationStrategy(c)
}

func (h *handler) requireOperationStrategyHandler(c *gin.Context) bool {
	if h.operationStrategyHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "operation_strategy_repository_unavailable",
		"message": "ListingKit operation strategy repository is not configured",
	})
	return false
}
