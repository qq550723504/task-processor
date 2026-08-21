package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingsubscription"
)

const (
	studioProductImageModule = listingsubscription.ModuleStudio
	studioProductImageMetric = "product_image_jobs"
)

type subscriptionStudioProductImageUsage struct {
	service *listingsubscription.Service
}

func studioProductImageUsageDependency(service *listingsubscription.Service) *subscriptionStudioProductImageUsage {
	if service == nil {
		return nil
	}
	return &subscriptionStudioProductImageUsage{service: service}
}

func (a *subscriptionStudioProductImageUsage) AuthorizeProductImageUsage(ctx context.Context, tenantID string, quantity int) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 {
		return fmt.Errorf("product image usage authorization requires tenant and positive quantity")
	}
	result, err := a.service.AuthorizeUsage(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, quantity)
	if err != nil {
		return err
	}
	if !result.Allowed {
		return listingsubscription.ErrSubscriptionRequired
	}
	return nil
}

func (a *subscriptionStudioProductImageUsage) RecordProductImageUsage(ctx context.Context, tenantID string, quantity int) error {
	if a == nil || a.service == nil {
		return listingsubscription.ErrSubscriptionRequired
	}
	if strings.TrimSpace(tenantID) == "" || quantity <= 0 {
		return fmt.Errorf("product image usage recording requires tenant and positive quantity")
	}
	_, err := a.service.RecordUsage(ctx, strings.TrimSpace(tenantID), studioProductImageModule, studioProductImageMetric, quantity)
	return err
}
