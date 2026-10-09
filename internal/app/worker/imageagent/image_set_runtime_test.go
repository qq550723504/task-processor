package imageagentworker

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
	"testing"
)

func TestImageSetExecutorDoesNotDispatchUnconfirmedOrSingleImageInputs(t *testing.T) {
	executor := imageSetSlotExecutor{}
	input := imageagent.SlotExecutionInput{}
	_, err := executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	_, err = executor.GenerateQuotedSlot(context.Background(), input, imageagent.SlotUsageQuote{})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	_, err = executor.ExecuteSlot(context.Background(), input)
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	_, err = executor.BuildSlotResult(context.Background(), input, imageagent.PublishedSlotOutput{})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	_, err = NewImageSetTemporalDependencies(nil, nil, nil, nil, nil, nil, nil)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}
