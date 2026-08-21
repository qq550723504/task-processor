package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func (h *handler) GenerateStudioProductImages(c *gin.Context) {
	var req listingkit.StudioProductImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	releaseCtx := studioProductImageUsageReleaseContext(c)
	ledgerAdmission := studioProductImageUsageLedgerEnabled(h)
	reservationID := ""
	if ledgerAdmission {
		var reserveErr error
		reservationID, reserveErr = h.reserveStudioProductImageUsage(c, "")
		if reserveErr != nil {
			writeStudioProductImageUsageAdmissionError(c, reserveErr)
			return
		}
	} else if !h.authorizeSubscriptionUsage(c, listingsubscription.ModuleStudio, "product_image_jobs", 1) {
		return
	}

	response, err := h.studioMediaService.GenerateStudioProductImages(requestContext(c), &req)
	if err != nil {
		if releaseErr := releaseStudioProductImageUsage(releaseCtx, h.subscriptionService, reservationID, "generation_failed"); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release product image usage: %w", releaseErr))
		}
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid request") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": "studio_product_images_failed", "message": err.Error()})
		return
	}
	for idx := range response.Images {
		response.Images[idx].ImageURL = publicizeUploadedImageURLs(c, []string{response.Images[idx].ImageURL})[0]
	}
	if ledgerAdmission {
		if commitErr := commitStudioProductImageUsage(requestContext(c), h.subscriptionService, reservationID); commitErr != nil {
			if releaseErr := releaseStudioProductImageUsage(releaseCtx, h.subscriptionService, reservationID, "commit_failed"); releaseErr != nil {
				commitErr = errors.Join(commitErr, fmt.Errorf("release product image usage: %w", releaseErr))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "studio_product_images_failed", "message": commitErr.Error()})
			return
		}
	} else {
		h.recordSubscriptionUsage(c, listingsubscription.ModuleStudio, "product_image_jobs", 1)
	}

	c.JSON(http.StatusOK, response)
}
