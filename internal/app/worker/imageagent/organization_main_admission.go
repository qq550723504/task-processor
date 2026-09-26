package imageagentworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/shared/aiidentity"
)

// organizationMainSlotExecutor is the #487 single-main-slot admission seam.
// Temporal owns the durable image effect; the existing commercial owner holds
// the bounded Review token reservation before any image provider dispatch.
type organizationMainSlotExecutor struct {
	delegate    organizationMainSlotDelegate
	quoter      productimage.UsageQuoter
	reservation aicapability.InvocationUsageReservation
}

type organizationMainSlotDelegate interface {
	imageagent.BudgetedStagedSlotExecutor
	imageagent.BudgetedStagedSlotReviewer
}

func (e organizationMainSlotExecutor) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	if e.delegate == nil {
		return imageagent.SlotUsageQuote{}, imageagent.ErrValidation
	}
	return e.delegate.QuoteSlot(ctx, input, policy)
}

func (e organizationMainSlotExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, expected imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	quote, invocationID, memberID, err := e.reviewAdmission(ctx, input, expected)
	if err != nil {
		return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: err}
	}
	if err := e.reservation.ReserveAIInvocationUsage(ctx, input.TenantID, memberID, invocationID, quote.MaximumTokens, time.Now().UTC()); err != nil {
		return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: err}
	}
	output, generateErr := e.delegate.GenerateQuotedSlot(withPreReservedReview(ctx, preReservedReview{InvocationID: invocationID, QuoteFingerprint: quote.Fingerprint}), input, expected)
	if generateErr != nil {
		switch imageagent.ProviderDispatchStateOf(generateErr) {
		case imageagent.ProviderNotDispatched, imageagent.ProviderRejectedBeforeEffect:
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			releaseErr := e.reservation.ReleaseAIInvocationUsage(releaseCtx, input.TenantID, invocationID)
			cancel()
			if releaseErr != nil {
				return output, &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: fmt.Errorf("release undispatched review reservation: %w", releaseErr)}
			}
		}
	}
	return output, generateErr
}

func (e organizationMainSlotExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: imageagent.ErrBudgetQuoteUnavailable}
}

func (e organizationMainSlotExecutor) BuildSlotResult(ctx context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	if e.delegate == nil {
		return imageagent.SlotExecutionResult{}, imageagent.ErrValidation
	}
	return e.delegate.BuildSlotResult(ctx, input, published)
}

func (e organizationMainSlotExecutor) QuoteStagedReview(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	if e.delegate == nil {
		return imageagent.SlotUsageQuote{}, imageagent.ErrValidation
	}
	return e.delegate.QuoteStagedReview(ctx, input, policy)
}

func (e organizationMainSlotExecutor) ReviewStagedSlotQuoted(ctx context.Context, input imageagent.SlotExecutionInput, staged imageagent.SlotGeneratedOutput, expected imageagent.SlotUsageQuote) (imageagent.SlotUsageReceipt, error) {
	quote, invocationID, _, err := e.reviewAdmission(ctx, input, expected)
	if err != nil {
		return imageagent.SlotUsageReceipt{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: err}
	}
	return e.delegate.ReviewStagedSlotQuoted(withPreReservedReview(ctx, preReservedReview{InvocationID: invocationID, QuoteFingerprint: quote.Fingerprint}), input, staged, expected)
}

func (e organizationMainSlotExecutor) ReviewStagedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput) error {
	return imageagent.ErrBudgetQuoteUnavailable
}

func (e organizationMainSlotExecutor) reviewAdmission(ctx context.Context, input imageagent.SlotExecutionInput, expected imageagent.SlotUsageQuote) (productimage.UsageQuote, string, string, error) {
	if input.TargetPlatform == "product" && input.ImagePolicyContext != nil &&
		*input.ImagePolicyContext == (imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}) {
		// This product flow no longer runs Review. Its image provider currently
		// has no approved token bound/settlement contract; do not borrow the
		// retired Review quote to admit generation or silently count it as zero.
		return productimage.UsageQuote{}, "", "", imageagent.ErrBudgetQuoteUnavailable
	}
	if e.delegate == nil || e.quoter == nil || e.reservation == nil || input.Slot.Role != imageagent.SlotRoleMain || input.PlanRevision <= 0 || input.Attempt <= 0 {
		return productimage.UsageQuote{}, "", "", imageagent.ErrValidation
	}
	identity := aiidentity.FromContext(ctx)
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || verified.EffectiveOrganizationID != input.TenantID || verified.TenantID != input.TenantID || verified.UserID != input.UserID || verified.EffectiveMemberID == "" || identity.AgentRunID != input.RunID || identity.TenantID != input.TenantID || identity.UserID != input.UserID {
		return productimage.UsageQuote{}, "", "", imageagent.ErrIdentityRequired
	}
	operationFingerprint := ""
	for _, operation := range expected.Operations {
		if operation.Name == "review" {
			if operationFingerprint != "" {
				return productimage.UsageQuote{}, "", "", imageagent.ErrRevisionConflict
			}
			operationFingerprint = operation.Fingerprint
		}
	}
	if operationFingerprint == "" {
		return productimage.UsageQuote{}, "", "", imageagent.ErrBudgetQuoteUnavailable
	}
	quoted, err := e.quoter.QuoteUsage(ctx, productimage.UsageQuoteRequest{Operation: "review", InputFingerprint: imageagent.SlotExecutionFingerprint(input), MaximumOutputs: 1})
	if err != nil {
		return productimage.UsageQuote{}, "", "", err
	}
	quoted, err = productimage.NormalizeUsageAuthorization(quoted, "review")
	if err != nil || quoted.Fingerprint != operationFingerprint || quoted.MaximumTokens <= 0 || quoted.MaximumModelCalls != 1 || quoted.MaximumOutputs != 1 {
		return productimage.UsageQuote{}, "", "", imageagent.ErrRevisionConflict
	}
	return quoted, stableOrganizationMainReviewInvocationID(input, quoted.Fingerprint), verified.EffectiveMemberID, nil
}

func stableOrganizationMainReviewInvocationID(input imageagent.SlotExecutionInput, quoteFingerprint string) string {
	payload := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s\x00%d\x00%s", input.TenantID, input.UserID, input.RunID, input.PlanRevision, input.Slot.ID, input.Attempt, quoteFingerprint)
	digest := sha256.Sum256([]byte(payload))
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("organization-main-review:"+hex.EncodeToString(digest[:]))).String()
}

type preReservedReview struct {
	InvocationID     string
	QuoteFingerprint string
}

type preReservedReviewContextKey struct{}

func withPreReservedReview(ctx context.Context, value preReservedReview) context.Context {
	return context.WithValue(ctx, preReservedReviewContextKey{}, value)
}

func preReservedReviewFromContext(ctx context.Context) preReservedReview {
	value, _ := ctx.Value(preReservedReviewContextKey{}).(preReservedReview)
	return value
}

var _ imageagent.BudgetedStagedSlotExecutor = organizationMainSlotExecutor{}
var _ imageagent.BudgetedStagedSlotReviewer = organizationMainSlotExecutor{}
