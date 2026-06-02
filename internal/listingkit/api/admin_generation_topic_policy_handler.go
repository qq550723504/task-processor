package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) ListAdminGenerationTopicPolicies(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	h.generationTopicPolicyHandler.ListGenerationTopicPolicies(c)
}

func (h *handler) GetAdminGenerationTopicPolicy(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	h.generationTopicPolicyHandler.GetGenerationTopicPolicy(c)
}

func (h *handler) CreateAdminGenerationTopicPolicy(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicPolicyHandler.CreateGenerationTopicPolicy(c)
}

func (h *handler) UpdateAdminGenerationTopicPolicy(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicPolicyHandler.UpdateGenerationTopicPolicy(c)
}

func (h *handler) UpdateAdminGenerationTopicPolicyStatus(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicPolicyHandler.UpdateGenerationTopicPolicyStatus(c)
}

func (h *handler) DeleteAdminGenerationTopicPolicy(c *gin.Context) {
	if !h.requireGenerationTopicPolicyHandler(c) {
		return
	}
	if !h.requireSubscription(c, listingsubscription.ModuleRules) {
		return
	}
	h.generationTopicPolicyHandler.DeleteGenerationTopicPolicy(c)
}

func (h *handler) requireGenerationTopicPolicyHandler(c *gin.Context) bool {
	if h.generationTopicPolicyHandler != nil {
		return true
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "generation_topic_policy_repository_unavailable",
		"message": "ListingKit generation topic policy repository is not configured",
	})
	return false
}
