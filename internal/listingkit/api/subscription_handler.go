package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func (h *handler) GetCurrentSubscription(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.GetCurrentSubscription(c)
}

func (h *handler) ListSubscriptionModules(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.ListModules(c)
}

func (h *handler) ListSubscriptionEntitlements(c *gin.Context) {
	if !h.requireSubscriptionHandler(c) {
		return
	}
	h.subscriptionHandler.ListEntitlements(c)
}

func (h *handler) writeSubscriptionPlanError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, listingsubscription.ErrModuleNotFound) {
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"error": "subscription_plan_update_failed", "message": err.Error()})
}
