package temporal

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
)

func TestSetApprovalRequiresExplicitCompleteSelection(t *testing.T) {
	_, repo, activityInput := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: activityInput.Identity.TenantID, OwnerUserID: activityInput.Identity.UserID, RunID: activityInput.RunID})
	require.NoError(t, err)
	input := WorkflowInput{RunID: current.Run.ID, Identity: activityInput.Identity, Plan: current.Plan}
	projection := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: current.Plan, ResultDigest: strings.Repeat("a", 64)}
	state := workflowUpdateState{input: &input, projection: &projection}
	signal := ApproveResultsSignal{RunID: input.RunID, PlanRevision: 1, ResultDigest: projection.ResultDigest, ActorID: input.Identity.UserID, ActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8"}
	require.Error(t, state.validateApproveResultsBusiness(signal), "set approval cannot silently approve all generated images")
	source := current.Plan.Set.Source
	signal.Selection = &productasset.ImageSetCommand{ActionID: signal.ActionID, Source: productasset.SourceSelectionRequest{ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, TargetPlatform: "product"}, SelectionDigest: strings.Repeat("b", 64), Choices: []productasset.ImageSetChoice{{Kind: "source", SourceID: "source-1", Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}}}
	require.NoError(t, state.validateApproveResultsBusiness(signal))
	signal.Selection.Source.EffectiveCatalogVersion++
	require.Error(t, state.validateApproveResultsBusiness(signal), "a different source version cannot enter the original approval saga")
}

type selectedSetApprovalResult struct {
	FirstError, CancelError, CompetingError, ChangedError, ResumeError string
	Status                                                             imageagent.RunStatus
}

func selectedSetApprovalWorkflow(ctx workflow.Context, input WorkflowInput, command ApproveResultsSignal) (selectedSetApprovalResult, error) {
	ctx = imageAgentActivityContext(ctx)
	input.externalEffectFinalization = true
	projection := WorkflowResult{Status: imageagent.RunStatusAwaitingFinalApproval, Plan: input.Plan, ResultDigest: command.ResultDigest}
	results := []SlotWorkflowResult{}
	state := newWorkflowUpdateState(ctx, &input, &projection, &results, newWorkflowEffectOwner(ctx))
	_, first := state.handleApproveResults(ctx, command)
	_, cancelled := state.handleCancel(ctx, CancelSignal{RunID: input.RunID, PlanRevision: input.Plan.Revision, ActorID: input.Identity.UserID, ActionID: "ca1aa25c-3661-4c80-8c85-e3fe93220f8f"})
	competing := command
	competing.ActionID = "69d61795-814c-4832-a17c-65ac6b3740b2"
	competing.Selection = imageagent.CloneImageSetCommand(command.Selection)
	competing.Selection.ActionID = competing.ActionID
	_, competitor := state.handleApproveResults(ctx, competing)
	changed := command
	changed.Selection = imageagent.CloneImageSetCommand(command.Selection)
	changed.Selection.Choices[0].SourceID = "other-source"
	_, changedErr := state.handleApproveResults(ctx, changed)
	_, resumed := state.handleResume(ctx, ResumeCommandInput{RunID: input.RunID, ActorID: input.Identity.UserID, ActionID: command.ActionID})
	return selectedSetApprovalResult{FirstError: workflowTestErrorString(first), CancelError: workflowTestErrorString(cancelled), CompetingError: workflowTestErrorString(competitor), ChangedError: workflowTestErrorString(changedErr), ResumeError: workflowTestErrorString(resumed), Status: projection.Status}, nil
}

func TestImageSetLostApprovalACKResumesExactOrderedSelection(t *testing.T) {
	_, repo, slotInput := imageSetPersistenceFixture(t)
	current, err := repo.GetProjection(context.Background(), imageagent.RunScope{TenantID: slotInput.Identity.TenantID, OwnerUserID: slotInput.Identity.UserID, RunID: slotInput.RunID})
	require.NoError(t, err)
	input := WorkflowInput{RunID: current.Run.ID, Identity: slotInput.Identity, Plan: current.Plan}
	source := current.Plan.Set.Source
	command := ApproveResultsSignal{RunID: input.RunID, PlanRevision: 1, ResultDigest: strings.Repeat("a", 64), ActorID: input.Identity.UserID, ActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8"}
	command.Selection = &productasset.ImageSetCommand{ActionID: command.ActionID, Source: productasset.SourceSelectionRequest{ItemID: source.OperationID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalVersion, EffectiveCatalogVersion: source.EffectiveVersion, TargetPlatform: "product"}, SelectionDigest: strings.Repeat("b", 64), Choices: []productasset.ImageSetChoice{{Kind: "source", SourceID: "source-1", Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}}}}
	env := newWorkflowEnv(t)
	env.RegisterWorkflow(selectedSetApprovalWorkflow)
	env.RegisterActivityWithOptions(func(context.Context, PublishImageSetActivityInput) error { return nil }, sdkactivity.RegisterOptions{Name: activityPublishApprovedImageSet})
	boundary := false
	env.OnActivity(activityPersistPendingCommand, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		receipt := activityInputFromArgs[PersistPendingCommandActivityInput](t, args).Receipt
		if receipt.Phase == imageagent.ImageSetApprovalPublicationStarted {
			require.Equal(t, command.Selection.SelectionDigest, receipt.SelectionDigest)
			require.Equal(t, command.ResultDigest, receipt.ResultDigest)
			boundary = true
		}
	}).Return(nil)
	checkOriginal := func(args mock.Arguments) {
		require.True(t, boundary, "persist the original boundary before the Asset operation")
		published := activityInputFromArgs[PublishImageSetActivityInput](t, args)
		require.Equal(t, *command.Selection, published.Selection)
		require.Equal(t, command.ResultDigest, published.ResultDigest)
	}
	env.OnActivity(activityPublishApprovedImageSet, mock.Anything, mock.Anything).Run(checkOriginal).Return(sdktemporal.NewNonRetryableApplicationError("Asset committed; acknowledgement lost", "publication_ack_unknown", nil)).Once()
	env.OnActivity(activityPublishApprovedImageSet, mock.Anything, mock.Anything).Run(checkOriginal).Return(nil).Once()
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil).Once()
	env.ExecuteWorkflow(selectedSetApprovalWorkflow, input, command)
	require.NoError(t, env.GetWorkflowError())
	var result selectedSetApprovalResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Contains(t, result.FirstError, "acknowledgement lost")
	require.Contains(t, result.CancelError, "approval publication")
	require.NotEmpty(t, result.CompetingError)
	require.NotEmpty(t, result.ChangedError)
	require.Empty(t, result.ResumeError)
	require.Equal(t, imageagent.RunStatusCompleted, result.Status)
	env.AssertExpectations(t)
}

func TestImageSetSelectionCommandsDoNotModifyCallerOwnedChoices(t *testing.T) {
	command := &productasset.ImageSetCommand{ActionID: "action", Choices: []productasset.ImageSetChoice{{Kind: "source", SourceID: "source", Presentation: productasset.ImagePresentation{Group: "detail", Order: 1}, OfficialPlacement: &productasset.ImageOfficialPlacement{Group: "skc", Type: 1, Sort: 1}}}, Target: &productasset.ImageSetTarget{StoreID: "store"}}
	copy := imageagent.CloneImageSetCommand(command)
	command.Choices[0].SourceID = "changed"
	command.Target.StoreID = "changed"
	command.Choices[0].OfficialPlacement.Type = 5
	require.Equal(t, "source", copy.Choices[0].SourceID)
	require.Equal(t, "store", copy.Target.StoreID)
	require.Equal(t, 1, copy.Choices[0].OfficialPlacement.Type)
}
