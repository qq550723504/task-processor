//go:build integration

package aiworkbenchpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/aiworkbench"
	"testing"
)

func TestNotificationPlanningFactsUseCompletePrivateKeyset(t *testing.T) {
	store, err := New(workbenchDB(t))
	require.NoError(t, err)
	ctx := context.Background()
	scope := aiworkbench.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	for i := 0; i < 121; i++ {
		readyTaskFixture(t, store, scope)
	}
	readyTaskFixture(t, store, aiworkbench.Scope{OrganizationID: "org-a", ActorID: "other-actor"})
	readyTaskFixture(t, store, aiworkbench.Scope{OrganizationID: "org-b", ActorID: "actor-a"})
	seen := map[string]bool{}
	after := ""
	for {
		rows, next, err := store.ListPlanningNotices(ctx, scope, after, 100)
		require.NoError(t, err)
		for _, row := range rows {
			require.Equal(t, scope, row.Command.Scope)
			require.NotEmpty(t, row.Command.ProposalID)
			require.False(t, seen[row.Command.PlannerInvocationID])
			seen[row.Command.PlannerInvocationID] = true
		}
		if next == "" {
			break
		}
		require.NotEqual(t, after, next)
		after = next
	}
	require.Len(t, seen, 121)
}
