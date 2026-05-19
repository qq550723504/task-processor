package temporal

import (
	"context"
	"fmt"

	sdkactivity "go.temporal.io/sdk/activity"

	"task-processor/internal/listingkit"
)

const (
	activityNameProcessStandardProduct = "ProcessStandardProduct"
	activityNameProcessPlatformAdapt   = "ProcessPlatformAdapt"
)

type LayerActivities struct {
	Host listingkit.LayerWorkflowActivityHost
}

func RegisterLayerActivities(reg activityRegisterer, activities *LayerActivities) error {
	if reg == nil {
		return fmt.Errorf("activity registerer is required")
	}
	if activities == nil {
		return fmt.Errorf("layer activities are required")
	}
	reg.RegisterActivityWithOptions(activities.ProcessStandardProduct, sdkactivity.RegisterOptions{Name: activityNameProcessStandardProduct})
	reg.RegisterActivityWithOptions(activities.ProcessPlatformAdaptation, sdkactivity.RegisterOptions{Name: activityNameProcessPlatformAdapt})
	return nil
}

func (a *LayerActivities) ProcessStandardProduct(ctx context.Context, in StandardProductWorkflowInput) (*listingkit.StandardProductSnapshot, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.ProcessStandardProductLayer(ctx, in.TaskID)
}

func (a *LayerActivities) ProcessPlatformAdaptation(ctx context.Context, in PlatformAdaptWorkflowInput) (*listingkit.ListingKitResult, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.ProcessPlatformAdaptationLayer(ctx, in.TaskID, in.Platform)
}

func (a *LayerActivities) host() (listingkit.LayerWorkflowActivityHost, error) {
	if a == nil || a.Host == nil {
		return nil, fmt.Errorf("layer activities host is not configured")
	}
	return a.Host, nil
}
