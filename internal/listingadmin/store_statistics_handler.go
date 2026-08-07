package listingadmin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type StoreStatisticsHandler struct {
	repo StoreStatisticsRepository
}

func NewStoreStatisticsHandler(repo StoreStatisticsRepository) *StoreStatisticsHandler {
	return &StoreStatisticsHandler{repo: repo}
}

func (h *StoreStatisticsHandler) ListStoreStatistics(c *gin.Context) {
	scope := requestListScope(c)
	if scope.TenantID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return
	}
	h.listStoreStatistics(c, scope)
}

func (h *StoreStatisticsHandler) ListPlatformStoreStatistics(c *gin.Context) {
	if !hasPlatformStoreAccess(c) {
		c.JSON(http.StatusForbidden, gin.H{"message": "platform store access is required"})
		return
	}
	h.listStoreStatistics(c, listQueryScope{})
}

func (h *StoreStatisticsHandler) listStoreStatistics(c *gin.Context, scope listQueryScope) {
	date, ok := queryDate(c, "date")
	if !ok {
		return
	}
	query := applyListQueryScope(&StoreStatisticsQuery{Date: date}, scope)
	page, err := h.repo.ListStoreStatistics(requestIdentityContext(c), *query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, page)
}
