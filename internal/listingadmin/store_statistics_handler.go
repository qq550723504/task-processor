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
	tenantID := requestTenantID(c)
	if tenantID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return
	}
	items, err := h.repo.ListStoreStatistics(requestIdentityContext(c), StoreStatisticsQuery{
		TenantID:    tenantID,
		OwnerUserID: requestScopedOwnerUserID(c),
		Date:        c.Query("date"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}
