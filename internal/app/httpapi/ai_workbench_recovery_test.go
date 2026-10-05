package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	"task-processor/internal/aiworkbench"
	governed "task-processor/internal/integration/aicapability/einomodel"
)

type workbenchInvocationReadFixture struct {
	ProductAgentInvocationLedger
	fact aicapability.InvocationRecord
	err  error
}

func (l workbenchInvocationReadFixture) ReadModelInvocation(context.Context, string, string) (aicapability.InvocationRecord, error) {
	return l.fact, l.err
}

func TestWorkbenchPlannerFailureRequiresAuthoritativeAbsenceForReclaim(t *testing.T) {
	for _, tc := range []struct {
		name      string
		readErr   error
		callErr   error
		remaining time.Duration
		want      aiworkbench.PlanningState
	}{
		{"absent within deadline", gorm.ErrRecordNotFound, governed.ErrOutcomeUnknown, time.Minute, aiworkbench.PlanningReadyToDispatch},
		{"absent after deadline", gorm.ErrRecordNotFound, governed.ErrOutcomeUnknown, -time.Second, aiworkbench.PlanningUnknown},
		{"read unavailable", errors.New("ledger unavailable"), governed.ErrOutcomeUnknown, time.Minute, aiworkbench.PlanningUnknown},
		{"known rejected before claim", gorm.ErrRecordNotFound, governed.ErrNotDispatched, time.Minute, aiworkbench.PlanningFailedBeforeDispatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := workbenchPlanner{agent: &productAgentApplication{config: ProductAgentDependencies{
				Ledger: workbenchInvocationReadFixture{err: tc.readErr},
			}}}
			command := aiworkbench.PlanningCommand{Scope: aiworkbench.Scope{OrganizationID: "org", ActorID: "actor"},
				PlannerInvocationID: "frozen-invocation", Deadline: time.Now().Add(tc.remaining)}
			require.Equal(t, tc.want, p.FailureState(context.Background(), command, tc.callErr))
		})
	}
}
