package einomodel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability"
)

func TestBoundedAdmissionSharesCredentialBurstAcrossOperations(t *testing.T) {
	admission := NewBoundedAdmission()
	input := aicapability.TextInputIdentity{OrganizationID: "org", Operation: aicapability.OperationAIWorkbenchChatPlan,
		Profile: aicapability.ModelProfile{ClientName: "shared-credential"}}
	for range 15 {
		release, err := admission.Acquire(context.Background(), input)
		require.NoError(t, err)
		release()
	}
	input.Operation = aicapability.OperationProductAgentDecision
	bounded, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := admission.Acquire(bounded, input)
	require.ErrorIs(t, err, ErrNotDispatched, "Chat and title use one credential burst budget")
	input.OrganizationID = "other-org"
	release, err := admission.Acquire(context.Background(), input)
	require.NoError(t, err, "one organization's budget must not consume another's")
	release()
}
