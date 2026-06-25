package httpapi

import "github.com/gin-gonic/gin"

type sheinSyncRouteHandler interface {
	ListSheinEnrollmentDashboard(c *gin.Context)
	TriggerSheinStoreSync(c *gin.Context)
	GetSheinEnrollmentStoreSummary(c *gin.Context)
	ListSheinSyncedProducts(c *gin.Context)
	UpdateSheinSyncedProductCost(c *gin.Context)
	ListSheinSDSCostGroups(c *gin.Context)
	ListSheinSourceSDSMetadata(c *gin.Context)
	UpdateSheinSDSCostGroup(c *gin.Context)
	RefreshSheinActivityCandidates(c *gin.Context)
	ListSheinActivityCandidates(c *gin.Context)
	ReviewSheinActivityCandidate(c *gin.Context)
	ExecuteSheinActivityEnrollment(c *gin.Context)
	ListSheinActivityEnrollmentRuns(c *gin.Context)
}
