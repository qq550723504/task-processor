package temporal

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/workflow"
	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
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
	if a.imageSetApprovals != nil && a.executionAuthorizer != nil {
		current, err := a.loadOrganizationExecution(ctx, input.RunID, input.Identity)
		if err != nil {
			return err
		}
		if input.PlanRevision != current.Plan.Revision || input.ResultDigest != current.ResultDigest {
			return imageagent.ErrRevisionConflict
		}
		if err := imageagent.ValidateImageSetSelectionIntent(input.RunID, current.Plan, input.ResultDigest, input.Selection.ActionID, &input.Selection); err != nil {
			return err
		}
		commit, committed, err := a.readCommittedImageSetApproval(ctx, current)
		if err != nil {
			return err
		}
		if committed {
			if commit.ActionID != input.Selection.ActionID || commit.ImageSet.Digest != input.Selection.SelectionDigest || commit.ImageSet.RequestDigest != productasset.ImageSetRequestDigest(input.Selection) {
				return imageagent.ErrRevisionConflict
			}
			return nil
		}
	}
	ctx, err := a.restoreExecutionIdentity(ctx, input.RunID, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.imageSetPublisher.PublishApprovedImageSet(ctx, imageagent.PublishImageSetInput{RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID, PlanRevision: input.PlanRevision, ResultDigest: input.ResultDigest, Selection: *imageagent.CloneImageSetCommand(&input.Selection)})
	return err
}
