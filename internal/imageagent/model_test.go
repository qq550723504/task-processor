package imageagent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePlanAllowsMoreThanTenIndependentSlots(t *testing.T) {
	slots := make([]Slot, 11)
	for i := range slots {
		slots[i] = Slot{ID: fmt.Sprintf("slot-%d", i), Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}}
	}
	err := ValidatePlan(Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: slots})
	require.NoError(t, err)
}

func TestValidatePlanRejectsDuplicateSlotIDs(t *testing.T) {
	plan := Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
		{ID: "scene-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}},
		{ID: "scene-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}},
	}}
	require.ErrorContains(t, ValidatePlan(plan), "duplicate slot id")
}

func TestAllowedActionsForBlockedRunAreExplicit(t *testing.T) {
	run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: "slot_failed", SlotID: "scene-2"}}
	require.Equal(t, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}, AllowedActions(run))
}

func TestAllowedActionsForTerminalRunAreEmpty(t *testing.T) {
	for _, status := range []RunStatus{RunStatusCompleted, RunStatusFailed, RunStatusCancelled} {
		require.Empty(t, AllowedActions(Run{Status: status}))
	}
}

func TestAllowedActionsForAmbiguousBlockedRunOnlyAllowCancel(t *testing.T) {
	require.Equal(t, []Action{ActionCancel}, AllowedActions(Run{Status: RunStatusBlocked}))
	require.Equal(t, []Action{ActionCancel}, AllowedActions(Run{Status: RunStatusBlocked, Block: &Block{}}))
}

func TestValidatePlanRejectsInvalidPlanInvariants(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		message string
	}{
		{name: "revision", plan: Plan{SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}}}}, message: "revision must be positive"},
		{name: "source assets", plan: Plan{Revision: 1, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene}}}, message: "at least one source asset"},
		{name: "slots", plan: Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}}, message: "at least one slot"},
		{name: "empty slot id", plan: Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}}}}, message: "slot id must not be empty"},
		{name: "unknown role", plan: Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: "unknown", SourceAssetIDs: []string{"source-1"}}}}, message: "unknown slot role"},
		{name: "duplicate idempotency key", plan: Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
			{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}},
			{ID: "slot-2", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}},
		}}, message: "duplicate idempotency key"},
		{name: "unknown source reference", plan: Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-2"}}}}, message: "source asset reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, ValidatePlan(tt.plan), tt.message)
		})
	}
}
