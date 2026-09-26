package imageagentworker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/shared/aiidentity"
)

func TestOrganizationMainRetiredReviewAdmissionCannotReserveAcrossGeneration(t *testing.T) {
	for _, platform := range []string{"product", "shein", ""} {
		t.Run(platform, func(t *testing.T) {
			delegate := &recordingMainExecutor{}
			executor := organizationMainSlotExecutor{delegate: delegate}
			input := mainAdmissionInput()
			input.TargetPlatform = platform
			input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
			if platform == "shein" {
				input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}
			}
			ctx, cancel := context.WithCancel(mainAdmissionContext())
			for _, cancelled := range []bool{false, true} {
				if cancelled {
					cancel()
				}
				_, err := executor.GenerateQuotedSlot(ctx, input, mainGenerationQuote())
				require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
				require.Equal(t, imageagent.ProviderNotDispatched, imageagent.ProviderDispatchStateOf(err))
				_, err = executor.GenerateSlot(ctx, input)
				require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
				_, err = executor.QuoteStagedReview(ctx, input, imageagent.BudgetPolicy{})
				require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
				_, err = executor.ReviewStagedSlotQuoted(ctx, input, imageagent.SlotGeneratedOutput{}, mainGenerationQuote())
				require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
				require.ErrorIs(t, executor.ReviewStagedSlot(ctx, input, imageagent.SlotGeneratedOutput{}), imageagent.ErrBudgetQuoteUnavailable)
			}
			cancel()
			require.Zero(t, delegate.generateCalls)
			require.Zero(t, delegate.reviewCalls)
		})
	}
}

func TestStandaloneReviewIdentityRemainsPayloadBoundWithoutPreReservationOverride(t *testing.T) {
	identity := aiidentity.FromContext(mainAdmissionContext())
	request := productimage.ReviewRequest{Sources: []productimage.Asset{{SourceAssetID: "source-1", URL: "https://example.com/source.jpg"}}, Candidates: []productimage.Candidate{{Asset: productimage.Asset{SourceAssetID: "source-1", Bytes: []byte("first")}}}}
	id, hash := stableReviewInvocationIdentity(identity, request, "quote-1")
	replayID, replayHash := stableReviewInvocationIdentity(identity, request, "quote-1")
	require.Equal(t, id, replayID)
	require.Equal(t, hash, replayHash)
	changedID, changedHash := stableReviewInvocationIdentity(identity, request, "quote-2")
	require.NotEqual(t, id, changedID)
	require.NotEqual(t, hash, changedHash)
	request.Candidates[0].Asset.Bytes = []byte("changed")
	changedID, changedHash = stableReviewInvocationIdentity(identity, request, "quote-1")
	require.NotEqual(t, id, changedID)
	require.NotEqual(t, hash, changedHash)
}

func TestReviewOutcomeDistinguishesUnknownFromObservedInvalidOutput(t *testing.T) {
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(errors.New("timeout"), false))
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(errors.New("timeout"), true))
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(nil, false))
	require.Equal(t, aicapability.InvocationUsageObservedFailed, reviewInvocationTerminalOutcome(productimage.ErrOutputValidation, true))
	require.Equal(t, aicapability.InvocationSucceeded, reviewInvocationTerminalOutcome(nil, true))
}

func mainAdmissionContext() context.Context {
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org-1", EffectiveOrganizationID: "org-1", EffectiveMemberID: "member-1", UserID: "actor-1"})
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{TenantID: "org-1", UserID: "actor-1", AgentRunID: "run-1", BusinessTaskID: "receipt-1"})
}
func mainAdmissionInput() imageagent.SlotExecutionInput {
	return imageagent.SlotExecutionInput{RunID: "run-1", TenantID: "org-1", UserID: "actor-1", PlanRevision: 1, Slot: imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain}, Attempt: 1}
}
func mainGenerationQuote() imageagent.SlotUsageQuote {
	return imageagent.SlotUsageQuote{Fingerprint: "old-slot-quote", Operations: []imageagent.SlotUsageOperation{{Name: "review", Fingerprint: "old-review-quote", MaximumOutputs: 1}}}
}

type recordingMainExecutor struct{ generateCalls, reviewCalls int }

func (e *recordingMainExecutor) GenerateQuotedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	e.generateCalls++
	return imageagent.SlotGeneratedOutput{}, nil
}
func (*recordingMainExecutor) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	return mainGenerationQuote(), nil
}
func (e *recordingMainExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.generateCalls++
	return imageagent.SlotGeneratedOutput{}, nil
}
func (*recordingMainExecutor) BuildSlotResult(context.Context, imageagent.SlotExecutionInput, imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, nil
}
func (e *recordingMainExecutor) ReviewStagedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput) error {
	e.reviewCalls++
	return nil
}
func (e *recordingMainExecutor) QuoteStagedReview(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	e.reviewCalls++
	return imageagent.SlotUsageQuote{}, nil
}
func (e *recordingMainExecutor) ReviewStagedSlotQuoted(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput, imageagent.SlotUsageQuote) (imageagent.SlotUsageReceipt, error) {
	e.reviewCalls++
	return imageagent.SlotUsageReceipt{}, nil
}
