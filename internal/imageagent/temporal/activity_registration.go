package temporal

import (
	"context"
	"fmt"

	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"

	"task-processor/internal/imageagent"
)

func (a *Activities) PublishApproved(ctx context.Context, input PublishApprovedActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.publisher.PublishApproved(ctx, imageagent.PublishApprovedInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
	return err
}

// PublishApprovedV3 is additive and intentionally not registered here. Task 6
// owns selecting and registering imageagent.publish_approved.v3.
func (a *Activities) PublishApprovedV3(ctx context.Context, input PublishApprovedV3ActivityInput) error {
	if a.publisherV3 == nil {
		return fmt.Errorf("image agent v3 approved asset publisher is required")
	}
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	_, err = a.publisherV3.PublishApprovedV3(ctx, imageagent.PublishApprovedV3Input{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
	return err
}

func legacyMigrationRequiredError() error {
	return sdktemporal.NewNonRetryableApplicationError(
		"legacy image agent workflow requires an explicit v2 migration or a new run",
		updateErrorLegacyMigrationRequired,
		nil,
	)
}

func (a *Activities) LegacyExecuteSlot(context.Context, LegacyExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistSlotResult(context.Context, LegacyPersistSlotResultActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistRunState(context.Context, LegacyPersistRunStateActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPlanRevision(context.Context, LegacyPersistPlanRevisionActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPendingCommand(context.Context, LegacyPersistPendingCommandActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPublishApproved(context.Context, LegacyPublishApprovedActivityInput) error {
	return legacyMigrationRequiredError()
}

type activityRegistrar interface {
	RegisterActivityWithOptions(interface{}, sdkactivity.RegisterOptions)
}

func RegisterActivities(registrar activityRegistrar, activities *Activities) error {
	return RegisterActivitiesForMode(registrar, activities, WorkerWireModeV2)
}

func RegisterActivitiesForMode(registrar activityRegistrar, activities *Activities, mode WorkerWireMode) error {
	if registrar == nil {
		return fmt.Errorf("temporal activity registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	if err := validateWorkerWireMode(mode); err != nil {
		return err
	}
	if mode == WorkerWireModeV2 {
		registrar.RegisterActivityWithOptions(activities.LegacyExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlotLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResultLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunStateLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevisionLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommandLegacy})
		registrar.RegisterActivityWithOptions(activities.LegacyPublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApprovedLegacy})
		registrar.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
		registrar.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	}
	registrar.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	registrar.RegisterActivityWithOptions(activities.PersistWorkflowFailure, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailure})
	registrar.RegisterActivityWithOptions(activities.PersistWorkflowFailureV2, sdkactivity.RegisterOptions{Name: activityPersistWorkflowFailureV2})
	registrar.RegisterActivityWithOptions(activities.PersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevision})
	registrar.RegisterActivityWithOptions(activities.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	if mode == WorkerWireModeV2 {
		registrar.RegisterActivityWithOptions(activities.PublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApproved})
	} else {
		registrar.RegisterActivityWithOptions(activities.ExecuteSlotV3, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})
		registrar.RegisterActivityWithOptions(activities.ReviewStagedSlotV3, sdkactivity.RegisterOptions{Name: activityReviewStagedSlotV3})
		registrar.RegisterActivityWithOptions(activities.StartEffectRecoveryV3, sdkactivity.RegisterOptions{Name: activityStartEffectRecoveryV3})
		registrar.RegisterActivityWithOptions(activities.RecoverEffectV3, sdkactivity.RegisterOptions{Name: activityRecoverEffectV3})
		registrar.RegisterActivityWithOptions(activities.PersistRecoveryBlockedEffectV3, sdkactivity.RegisterOptions{Name: activityPersistRecoveryBlockedV3})
		registrar.RegisterActivityWithOptions(activities.ReconcileEffectRecoveryV3, sdkactivity.RegisterOptions{Name: activityReconcileEffectRecoveryV3})
		registrar.RegisterActivityWithOptions(activities.PersistSlotResultV3, sdkactivity.RegisterOptions{Name: activityPersistSlotResultV3})
		registrar.RegisterActivityWithOptions(activities.PublishApprovedV3, sdkactivity.RegisterOptions{Name: activityPublishApprovedV3})
	}
	return nil
}
