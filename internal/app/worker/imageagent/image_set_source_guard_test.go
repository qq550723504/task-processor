package imageagentworker

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/imageagent"
	"testing"
)

type dispatchSourceContexts struct {
	calls    int
	denied   error
	identity imageagent.ExecutionIdentity
}

func (*dispatchSourceContexts) ResolveImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	panic("dispatch must revalidate its original persisted plan")
}
func (c *dispatchSourceContexts) AuthorizeImageSetSource(_ context.Context, id imageagent.ExecutionIdentity, _ imageagent.RunProjection) error {
	c.calls++
	c.identity = id
	return c.denied
}

func TestSetDispatchRequiresFreshOriginalSourceOwnerBeforeReadingProviderInput(t *testing.T) {
	set := &imageagent.ImageSetPlan{Source: imageagent.ImageSourceBinding{ContextKind: imageagent.ImageSourceAcquisition, OperationID: "operation"}}
	input := imageagent.SlotExecutionInput{ImageSet: set, OrganizationIdentity: imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "original-member", BusinessTaskID: "operation"}}
	repo := generationProjectionReader{plan: imageagent.Plan{Set: set}}
	require.ErrorIs(t, revalidateGenerationImageSet(context.Background(), repo, nil, input), imageagent.ErrCommandBlocked)
	contexts := &dispatchSourceContexts{denied: imageagent.ErrRevisionConflict}
	require.ErrorIs(t, revalidateGenerationImageSet(context.Background(), repo, contexts, input), imageagent.ErrRevisionConflict)
	require.Equal(t, 1, contexts.calls)
	require.Equal(t, input.OrganizationIdentity, contexts.identity)
	altered := *set
	altered.Source.ContextKind = imageagent.ImageSourceSupply
	input.ImageSet = &altered
	require.ErrorIs(t, revalidateGenerationImageSet(context.Background(), repo, contexts, input), imageagent.ErrRevisionConflict)
	require.Equal(t, 1, contexts.calls, "an altered source owner cannot reach source fetch")
}
