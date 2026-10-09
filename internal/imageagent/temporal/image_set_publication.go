package temporal

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/workflow"
	"task-processor/internal/imageagent"
)

func (o *workflowEffectOwner) publishImageSet(ctx workflow.Context, input PublishImageSetActivityInput) error {
	return o.execute(ctx, "", func(ownerCtx workflow.Context) error {
		if err := workflow.ExecuteActivity(ownerCtx, activityPublishApprovedImageSet, input).Get(ownerCtx, nil); err != nil {
			return fmt.Errorf("publish selected image set: %w", err)
		}
		return nil
	})
}

func (a *Activities) PublishApprovedImageSet(ctx context.Context, input PublishImageSetActivityInput) error {
	if a.imageSetPublisher == nil {
		return imageagent.ErrCommandBlocked
	}
	ctx, err := a.restoreExecutionIdentity(ctx, input.RunID, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.imageSetPublisher.PublishApprovedImageSet(ctx, imageagent.PublishImageSetInput{RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID, PlanRevision: input.PlanRevision, ResultDigest: input.ResultDigest, Selection: *imageagent.CloneImageSetCommand(&input.Selection)})
	return err
}
