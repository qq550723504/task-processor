package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit/sheinpodimage"
)

type lookupSheinPODImagesQuery struct {
	Query string `form:"query"`
	Limit int    `form:"limit"`
}

func (h *handler) LookupSheinPODImages(c *gin.Context) {
	if h.sheinPODImageLookupService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "shein_pod_image_lookup_unavailable", "message": "SHEIN POD image lookup service is not configured"})
		return
	}
	storeID, err := parseSheinInt64Param(c, "store_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	var query lookupSheinPODImagesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	query.Query = strings.TrimSpace(query.Query)
	if query.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "query is required"})
		return
	}
	items, total, err := h.sheinPODImageLookupService.LookupSheinPODImages(requestContext(c), &sheinpodimage.SheinPODImageLookupQuery{
		StoreID: storeID,
		Query:   query.Query,
		Limit:   query.Limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "shein_pod_image_lookup_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}
