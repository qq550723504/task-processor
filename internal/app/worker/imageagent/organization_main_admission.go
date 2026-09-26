package imageagentworker

import (
	"context"

	"task-processor/internal/imageagent"
)

// organizationMainSlotExecutor is the current #487 generation admission seam.
// The cancelled Extract/Render/Review flow has no production consumer. In
// particular, this executor must never reserve tokens for a future Review.
// Generation remains closed until its own metering contract is available.
type organizationMainSlotExecutor struct {
	delegate imageagent.BudgetedStagedSlotExecutor
}

func (e organizationMainSlotExecutor) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, policy imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	if e.delegate == nil {
		return imageagent.SlotUsageQuote{}, imageagent.ErrValidation
	}
	return e.delegate.QuoteSlot(ctx, input, policy)
}

func (organizationMainSlotExecutor) GenerateQuotedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: imageagent.ErrBudgetQuoteUnavailable}
}

func (organizationMainSlotExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: imageagent.ErrBudgetQuoteUnavailable}
}

func (e organizationMainSlotExecutor) BuildSlotResult(ctx context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	if e.delegate == nil {
		return imageagent.SlotExecutionResult{}, imageagent.ErrValidation
	}
	return e.delegate.BuildSlotResult(ctx, input, published)
}

// Old staged-review attempts cannot reopen the retired cross-step reservation.
// Existing durable records are neither released nor reinterpreted here.
func (organizationMainSlotExecutor) QuoteStagedReview(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	return imageagent.SlotUsageQuote{}, imageagent.ErrBudgetQuoteUnavailable
}

func (organizationMainSlotExecutor) ReviewStagedSlotQuoted(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput, imageagent.SlotUsageQuote) (imageagent.SlotUsageReceipt, error) {
	return imageagent.SlotUsageReceipt{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderNotDispatched, Err: imageagent.ErrBudgetQuoteUnavailable}
}

func (organizationMainSlotExecutor) ReviewStagedSlot(context.Context, imageagent.SlotExecutionInput, imageagent.SlotGeneratedOutput) error {
	return imageagent.ErrBudgetQuoteUnavailable
}

var _ imageagent.BudgetedStagedSlotExecutor = organizationMainSlotExecutor{}
var _ imageagent.BudgetedStagedSlotReviewer = organizationMainSlotExecutor{}
