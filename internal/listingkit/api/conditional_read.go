package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func applyGenerationConditionalReadHeaders(c *gin.Context, query *listingkit.GenerationQueueQuery) {
	if c == nil || query == nil {
		return
	}
	if strings.TrimSpace(query.IfMatch) != "" || strings.TrimSpace(query.DeltaToken) != "" {
		return
	}
	ifMatch := normalizeGenerationETag(c.GetHeader("If-None-Match"))
	if ifMatch != "" {
		query.IfMatch = ifMatch
	}
}

func applyGenerationConditionalReadHeadersToNavigationTarget(c *gin.Context, req *listingkit.GenerationReviewNavigationDispatchRequest) {
	if c == nil || req == nil || req.Target == nil {
		return
	}
	listingkit.ApplyGenerationConditionalBaselineToNavigationTarget(req.Target, normalizeGenerationETag(c.GetHeader("If-None-Match")))
}

func applyGenerationConditionalReadHeadersToQuery(query *listingkit.GenerationQueueQuery, ifMatch string) {
	if query == nil {
		return
	}
	if strings.TrimSpace(query.IfMatch) != "" || strings.TrimSpace(query.DeltaToken) != "" {
		return
	}
	query.IfMatch = ifMatch
}

func writeGenerationConditionalReadResponse(c *gin.Context, deltaToken string, notModified bool, body any) {
	if c == nil {
		return
	}
	if token := strings.TrimSpace(deltaToken); token != "" {
		c.Header("ETag", formatGenerationETag(token))
	}
	if notModified && normalizeGenerationETag(c.GetHeader("If-None-Match")) != "" {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, body)
}

func writeGenerationConditionalDispatchResponse(c *gin.Context, deltaToken string, body any) {
	writeGenerationConditionalMutationResponse(c, deltaToken, body)
}

func writeGenerationConditionalMutationResponse(c *gin.Context, deltaToken string, body any) {
	if c == nil {
		return
	}
	if token := strings.TrimSpace(deltaToken); token != "" {
		c.Header("ETag", formatGenerationETag(token))
	}
	c.JSON(http.StatusOK, body)
}

func formatGenerationETag(token string) string {
	return `"` + strings.TrimSpace(token) + `"`
}

func normalizeGenerationETag(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "W/")
	value = strings.Trim(value, `"`)
	return strings.TrimSpace(value)
}
