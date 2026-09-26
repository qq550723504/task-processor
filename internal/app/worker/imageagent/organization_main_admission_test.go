package imageagentworker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/shared/aiidentity"
)

func TestOrganizationMainAdmissionReservesBeforeFirstProviderDispatch(t *testing.T) {
	quotaErr := errors.New("member quota exhausted")
	reservation := &recordingMainReservation{reserveErr: quotaErr}
	delegate := &recordingMainExecutor{}
	quoter := fixedMainReviewQuoter{quote: mainReviewQuote()}
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: quoter, reservation: reservation}
	input := mainAdmissionInput()
	_, err := executor.GenerateQuotedSlot(mainAdmissionContext(), input, mainGenerationQuote())
	require.ErrorIs(t, err, quotaErr)
	require.Equal(t, 0, delegate.generateCalls, "first image provider must not dispatch without member reservation")
	require.Equal(t, 1, reservation.reserveCalls)
	require.Equal(t, int64(48), reservation.maximumTokens)
	require.Equal(t, "member-1", reservation.memberID)
}

func TestGenericMainAdmissionCannotBorrowRetiredReviewTokenQuote(t *testing.T) {
	reservation := &recordingMainReservation{}
	delegate := &recordingMainExecutor{}
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: fixedMainReviewQuoter{quote: mainReviewQuote()}, reservation: reservation}
	input := mainAdmissionInput()
	input.TargetPlatform = "product"
	input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
	_, err := executor.GenerateQuotedSlot(mainAdmissionContext(), input, mainGenerationQuote())
	require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
	require.Zero(t, reservation.reserveCalls)
	require.Zero(t, delegate.generateCalls)
	require.Equal(t, imageagent.ProviderNotDispatched, imageagent.ProviderDispatchStateOf(err))
}

func TestOrganizationMainAdmissionUsesStableReviewIdentityAndRetainsUnknown(t *testing.T) {
	reservation := &recordingMainReservation{}
	delegate := &recordingMainExecutor{generateErr: &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: errors.New("unknown")}}
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: fixedMainReviewQuoter{quote: mainReviewQuote()}, reservation: reservation}
	input := mainAdmissionInput()
	_, err := executor.GenerateQuotedSlot(mainAdmissionContext(), input, mainGenerationQuote())
	require.Error(t, err)
	require.Equal(t, 1, delegate.generateCalls)
	require.Equal(t, 0, reservation.releaseCalls, "unknown image effect retains member reservation")
	require.Equal(t, stableOrganizationMainReviewInvocationID(input, "quote-review-1"), reservation.invocationID)
	require.Equal(t, reservation.invocationID, delegate.preReservedID)
}

func TestOrganizationMainAdmissionReleasesOnlyDefinitiveNonDispatch(t *testing.T) {
	reservation := &recordingMainReservation{}
	delegate := &recordingMainExecutor{generateErr: &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: errors.New("quote drift")}}
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: fixedMainReviewQuoter{quote: mainReviewQuote()}, reservation: reservation}
	_, err := executor.GenerateQuotedSlot(mainAdmissionContext(), mainAdmissionInput(), mainGenerationQuote())
	require.Error(t, err)
	require.Equal(t, 1, reservation.releaseCalls)
}

func TestOrganizationMainAdmissionRejectsReviewQuoteDriftBeforeReservation(t *testing.T) {
	reservation := &recordingMainReservation{}
	delegate := &recordingMainExecutor{}
	quote := mainReviewQuote()
	quote.Fingerprint = "new-route"
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: fixedMainReviewQuoter{quote: quote}, reservation: reservation}
	_, err := executor.GenerateQuotedSlot(mainAdmissionContext(), mainAdmissionInput(), mainGenerationQuote())
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Equal(t, 0, reservation.reserveCalls)
	require.Equal(t, 0, delegate.generateCalls)
}

func TestPreReservedReviewIdentityKeepsInvocationStableAndPayloadConflicts(t *testing.T) {
	input := mainAdmissionInput()
	reservedID := stableOrganizationMainReviewInvocationID(input, "quote-review-1")
	ctx := withPreReservedReview(mainAdmissionContext(), preReservedReview{InvocationID: reservedID, QuoteFingerprint: "quote-review-1"})
	identity := aiidentity.FromContext(ctx)
	first := productimage.ReviewRequest{Sources: []productimage.Asset{{SourceAssetID: "catalog-image-1", URL: "https://example.com/source.jpg"}}, Candidates: []productimage.Candidate{{Asset: productimage.Asset{SourceAssetID: "catalog-image-1", URL: "https://example.com/first.jpg"}}}}
	second := productimage.ReviewRequest{Sources: first.Sources, Candidates: []productimage.Candidate{{Asset: productimage.Asset{SourceAssetID: "catalog-image-1", URL: "https://example.com/changed.jpg"}}}}
	firstID, firstHash, err := reviewInvocationIdentity(ctx, identity, first, "quote-review-1")
	require.NoError(t, err)
	secondID, secondHash, err := reviewInvocationIdentity(ctx, identity, second, "quote-review-1")
	require.NoError(t, err)
	require.Equal(t, reservedID, firstID)
	require.Equal(t, firstID, secondID)
	require.NotEqual(t, firstHash, secondHash)
	_, _, err = reviewInvocationIdentity(ctx, identity, first, "changed-route")
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	firstInline := productimage.ReviewRequest{Sources: first.Sources, Candidates: []productimage.Candidate{{Asset: productimage.Asset{SourceAssetID: "catalog-image-1", Bytes: []byte("first generated image")}}}}
	secondInline := productimage.ReviewRequest{Sources: first.Sources, Candidates: []productimage.Candidate{{Asset: productimage.Asset{SourceAssetID: "catalog-image-1", Bytes: []byte("different generated image")}}}}
	_, firstInlineHash, err := reviewInvocationIdentity(ctx, identity, firstInline, "quote-review-1")
	require.NoError(t, err)
	_, secondInlineHash, err := reviewInvocationIdentity(ctx, identity, secondInline, "quote-review-1")
	require.NoError(t, err)
	require.NotEqual(t, firstInlineHash, secondInlineHash, "inline candidate bytes must bind the invocation without persistence")
}

func TestReviewOutcomeDistinguishesUnknownFromObservedInvalidOutput(t *testing.T) {
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(errors.New("timeout"), false))
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(errors.New("timeout"), true), "observer may have been called with a response but a transport error remains unknown")
	require.Equal(t, aicapability.InvocationDispatched, reviewInvocationTerminalOutcome(nil, false), "missing usage cannot be successful settlement")
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

func mainReviewQuote() productimage.UsageQuote {
	return productimage.UsageQuote{Operation: "review", Provider: "controlled", RouteReference: "route-1", Model: "review-model", CredentialReference: "credential-1", ConfigurationVersion: "config-1", PricingVersion: "pricing-1", Fingerprint: "quote-review-1", MaximumOutputs: 1, MaximumModelCalls: 1, MaximumTokens: 48}
}

func mainGenerationQuote() imageagent.SlotUsageQuote {
	return imageagent.SlotUsageQuote{Fingerprint: "slot-quote-1", Operations: []imageagent.SlotUsageOperation{{Name: "review", Fingerprint: "quote-review-1", MaximumOutputs: 1}}}
}

type fixedMainReviewQuoter struct{ quote productimage.UsageQuote }

func (q fixedMainReviewQuoter) QuoteUsage(context.Context, productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	return q.quote, nil
}

type recordingMainReservation struct {
	reserveErr    error
	reserveCalls  int
	releaseCalls  int
	maximumTokens int64
	memberID      string
	invocationID  string
}

func (r *recordingMainReservation) ReserveAIInvocationUsage(_ context.Context, _, memberID, invocationID string, maximumTokens int64, _ time.Time) error {
	r.reserveCalls++
	r.memberID, r.invocationID, r.maximumTokens = memberID, invocationID, maximumTokens
	return r.reserveErr
}

func (r *recordingMainReservation) ReleaseAIInvocationUsage(context.Context, string, string) error {
	r.releaseCalls++
	return nil
}

type recordingMainExecutor struct {
	generateCalls int
	generateErr   error
	preReservedID string
}

func (e *recordingMainExecutor) GenerateQuotedSlot(ctx context.Context, _ imageagent.SlotExecutionInput, _ imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	e.generateCalls++
	e.preReservedID = preReservedReviewFromContext(ctx).InvocationID
	return imageagent.SlotGeneratedOutput{}, e.generateErr
}

func (e *recordingMainExecutor) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	return mainGenerationQuote(), nil
}

func (e *recordingMainExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{}, nil
}

func (e *recordingMainExecutor) BuildSlotResult(context.Context, imageagent.SlotExecutionInput, imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, nil
}

func (e *recordingMainExecutor) ReviewStagedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput) error {
	return nil
}

func (e *recordingMainExecutor) QuoteStagedReview(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	return imageagent.SlotUsageQuote{}, nil
}

func (e *recordingMainExecutor) ReviewStagedSlotQuoted(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput, imageagent.SlotUsageQuote) (imageagent.SlotUsageReceipt, error) {
	return imageagent.SlotUsageReceipt{}, nil
}
