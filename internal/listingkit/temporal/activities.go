package temporal

import (
	"context"
	"fmt"
	"strings"

	sdkactivity "go.temporal.io/sdk/activity"

	"task-processor/internal/listingkit"
)

type SubmitActivities struct {
	Host listingkit.SheinPublishActivityHost
}

const (
	activityNameBeginPublishAttempt = "BeginPublishAttempt"
	activityNameValidateReadiness   = "ValidateReadiness"
	activityNamePrepareProduct      = "PrepareProduct"
	activityNameUploadImages        = "UploadImages"
	activityNamePreValidate         = "PreValidate"
	activityNameSubmitRemote        = "SubmitRemote"
	activityNamePersistSuccess      = "PersistSuccess"
	activityNamePersistFailure      = "PersistFailure"
	activityNameRefreshRemoteStatus = "RefreshRemoteStatus"
)

type activityRegisterer interface {
	RegisterActivityWithOptions(a interface{}, options sdkactivity.RegisterOptions)
}

func RegisterSubmitActivities(reg activityRegisterer, activities *SubmitActivities) error {
	if reg == nil {
		return fmt.Errorf("activity registerer is required")
	}
	if activities == nil {
		return fmt.Errorf("submit activities are required")
	}
	reg.RegisterActivityWithOptions(activities.BeginPublishAttempt, sdkactivity.RegisterOptions{Name: activityNameBeginPublishAttempt})
	reg.RegisterActivityWithOptions(activities.ValidateReadiness, sdkactivity.RegisterOptions{Name: activityNameValidateReadiness})
	reg.RegisterActivityWithOptions(activities.PrepareProduct, sdkactivity.RegisterOptions{Name: activityNamePrepareProduct})
	reg.RegisterActivityWithOptions(activities.UploadImages, sdkactivity.RegisterOptions{Name: activityNameUploadImages})
	reg.RegisterActivityWithOptions(activities.PreValidate, sdkactivity.RegisterOptions{Name: activityNamePreValidate})
	reg.RegisterActivityWithOptions(activities.SubmitRemote, sdkactivity.RegisterOptions{Name: activityNameSubmitRemote})
	reg.RegisterActivityWithOptions(activities.PersistSuccess, sdkactivity.RegisterOptions{Name: activityNamePersistSuccess})
	reg.RegisterActivityWithOptions(activities.PersistFailure, sdkactivity.RegisterOptions{Name: activityNamePersistFailure})
	reg.RegisterActivityWithOptions(activities.RefreshRemoteStatus, sdkactivity.RegisterOptions{Name: activityNameRefreshRemoteStatus})
	return nil
}

func (a *SubmitActivities) BeginPublishAttempt(ctx context.Context, in SheinPublishWorkflowInput) error {
	host, err := a.host()
	if err != nil {
		return err
	}
	return host.BeginSheinPublishAttempt(ctx, sheinPublishAttemptInput(in))
}

func (a *SubmitActivities) ValidateReadiness(ctx context.Context, in SheinPublishWorkflowInput) error {
	host, err := a.host()
	if err != nil {
		return err
	}
	return host.ValidateSheinPublishReadiness(ctx, sheinPublishAttemptInput(in))
}

func (a *SubmitActivities) PrepareProduct(ctx context.Context, in SheinPublishWorkflowInput) (*listingkit.SheinPreparedSubmitPayload, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.PrepareSheinPublishPayload(ctx, sheinPublishAttemptInput(in))
}

func (a *SubmitActivities) UploadImages(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinPreparedSubmitPayload, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.UploadSheinPublishImages(ctx, in)
}

func (a *SubmitActivities) PreValidate(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) error {
	host, err := a.host()
	if err != nil {
		return err
	}
	return host.PreValidateSheinPublish(ctx, in)
}

func (a *SubmitActivities) SubmitRemote(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinRemoteSubmitResult, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.SubmitSheinPublishRemote(ctx, in)
}

func (a *SubmitActivities) PersistSuccess(ctx context.Context, in listingkit.SheinPersistSubmitSuccessInput) error {
	host, err := a.host()
	if err != nil {
		return err
	}
	return host.PersistSheinPublishSuccess(ctx, in)
}

func (a *SubmitActivities) PersistFailure(ctx context.Context, in *listingkit.SheinPersistSubmitFailureInput) error {
	host, err := a.host()
	if err != nil {
		return err
	}
	if in == nil {
		return fmt.Errorf("shein publish failure input is required")
	}
	return host.PersistSheinPublishFailure(ctx, *in)
}

func (a *SubmitActivities) RefreshRemoteStatus(ctx context.Context, in listingkit.SheinRefreshRemoteStatusInput) (*listingkit.SheinRefreshRemoteStatusResult, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.RefreshSheinPublishRemoteStatus(ctx, in)
}

func (a *SubmitActivities) BuildPreview(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	host, err := a.host()
	if err != nil {
		return nil, err
	}
	return host.BuildSheinTaskPreview(ctx, strings.TrimSpace(taskID))
}

func (a *SubmitActivities) host() (listingkit.SheinPublishActivityHost, error) {
	if a == nil || a.Host == nil {
		return nil, fmt.Errorf("submit activities host is not configured")
	}
	return a.Host, nil
}

func sheinPublishAttemptInput(in SheinPublishWorkflowInput) listingkit.SheinPublishAttemptInput {
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "publish"
	}
	return listingkit.SheinPublishAttemptInput{
		TaskID:         strings.TrimSpace(in.TaskID),
		Action:         action,
		RequestID:      strings.TrimSpace(in.RequestID),
		ConfirmedFinal: in.ConfirmedFinal,
		RequestedAt:    in.RequestedAt,
	}
}
