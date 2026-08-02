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
	platformAccess := hasPlatformStoreAccess(c)
	if platformAccess {
		scope.TenantID = 0
		scope.OwnerUserID = ""
	}
	if scope.TenantID <= 0 && !platformAccess {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "tenant id is required"})
		return
	}
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
