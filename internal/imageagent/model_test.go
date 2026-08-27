package imageagent

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAssetCatalogPreservesExplicitStableManifest(t *testing.T) {
	createdAt := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)
	assets := []AuthorizedAsset{{ID: "source-1", Type: AuthorizedAssetSource, URL: "https://source.example/source.png", SourceURL: "https://source.example/source.png"}}
	manifest := CatalogManifest{Version: 7, Hash: CatalogHash(assets), CreatedAt: createdAt}

	first, err := NormalizeAssetCatalog(AssetCatalog{Manifest: manifest, Assets: assets})
	require.NoError(t, err)
	second, err := NormalizeAssetCatalog(first)
	require.NoError(t, err)
	require.Equal(t, manifest, first.Manifest)
	require.Equal(t, first, second)

	withoutCreationTime, err := NormalizeAssetCatalog(AssetCatalog{Assets: assets})
	require.NoError(t, err)
	require.True(t, withoutCreationTime.Manifest.CreatedAt.IsZero(), "normalization must not manufacture a repository-local clock")
}

func TestValidatePlanAllowsMoreThanTenIndependentSlots(t *testing.T) {
	slots := make([]Slot, 11)
	for i := range slots {
		slots[i] = Slot{ID: fmt.Sprintf("slot-%d", i), Role: SlotRoleScene, IdempotencyKey: fmt.Sprintf("slot-key-%d", i), SourceAssetIDs: []string{"source-1"}}
	}
	err := ValidatePlan(Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: slots})
	require.NoError(t, err)
}

func TestValidatePlanRejectsDuplicateSlotIDs(t *testing.T) {
	plan := Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
		{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key-1", SourceAssetIDs: []string{"source-1"}},
		{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key-2", SourceAssetIDs: []string{"source-1"}},
	}}
	plan.IdempotencyKey = "plan-key-1"
	require.ErrorContains(t, ValidatePlan(plan), "duplicate slot id")
}

func TestAllowedActionsForBlockedRunAreExplicit(t *testing.T) {
	run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: "slot_failed", SlotID: "scene-2"}}
	require.Equal(t, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}, AllowedActions(run))
}

func TestSlotEffectV3BlockedPolicyMapsExactPhaseCodeAndActions(t *testing.T) {
	for _, tc := range []struct {
		phase   SlotEffectV3Phase
		code    string
		actions []Action
	}{
		{SlotEffectV3ProviderUnknown, SlotProviderOutcomeUnknownCode, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}},
		{SlotEffectV3StagingUnknown, SlotStagingOutcomeUnknownCode, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}},
		{SlotEffectV3PublicationUnknown, SlotPublicationOutcomeUnknownCode, []Action{ActionEditPlan, ActionCancel}},
	} {
		policy, err := SlotEffectV3BlockedPolicyFor(tc.phase, tc.code)
		require.NoError(t, err)
		require.Equal(t, tc.actions, policy.PermittedActions)
		run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: tc.code, SlotID: "scene-2"}}
		require.Equal(t, tc.actions, AllowedActions(run))
	}
}

func TestSlotEffectV3BlockedPolicyRejectsUnknownAndMismatchedCodes(t *testing.T) {
	for _, tc := range []struct {
		phase SlotEffectV3Phase
		code  string
	}{
		{SlotEffectV3PublicationUnknown, SlotProviderOutcomeUnknownCode},
		{SlotEffectV3ProviderUnknown, "future_policy"},
		{SlotEffectV3Phase("future_phase"), SlotProviderOutcomeUnknownCode},
	} {
		_, err := SlotEffectV3BlockedPolicyFor(tc.phase, tc.code)
		require.ErrorIs(t, err, ErrInvalidPersistedPolicy)
	}
}

func TestAllowedActionsKeepsLegacySlotFailedBehavior(t *testing.T) {
	run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: "slot_failed", SlotID: "scene-2"}}
	require.Equal(t, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}, AllowedActions(run))
}

func TestAllowedActionsExposeFinalApprovalAndCancellation(t *testing.T) {
	run := Run{Mode: RunModeManual, Status: RunStatusAwaitingFinalApproval}
	require.Equal(t, []Action{ActionApproveResults, ActionCancel}, AllowedActions(run))
}

func TestAllowedActionsForTerminalRunAreEmpty(t *testing.T) {
	for _, status := range []RunStatus{RunStatusCompleted, RunStatusFailed, RunStatusCancelled} {
		require.Empty(t, AllowedActions(Run{Status: status}))
	}
}

func TestAllowedActionsForAmbiguousBlockedRunOnlyAllowCancel(t *testing.T) {
	require.Equal(t, []Action{ActionCancel}, AllowedActions(Run{Mode: RunModeManual, Status: RunStatusBlocked}))
	require.Equal(t, []Action{ActionCancel}, AllowedActions(Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{}}))
}

func TestAllowedActionsForNonManualRunAreEmpty(t *testing.T) {
	for _, mode := range []RunMode{RunModeAssisted, RunModeAutomatic} {
		run := Run{Mode: mode, Status: RunStatusBlocked, Block: &Block{Code: "slot_failed", SlotID: "scene-2"}}
		require.Empty(t, AllowedActions(run))
	}
}

func TestValidatePlanRejectsMissingPlanIdempotencyKey(t *testing.T) {
	plan := Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-1"}}}}
	require.ErrorContains(t, ValidatePlan(plan), "plan idempotency key")
}

func TestValidatePlanRejectsMissingSlotIdempotencyKey(t *testing.T) {
	plan := Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}}}}
	require.ErrorContains(t, ValidatePlan(plan), "slot idempotency key")
}

func TestValidatePlanRejectsInvalidPlanInvariants(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		message string
	}{
		{name: "revision", plan: Plan{IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-1"}}}}, message: "revision must be positive"},
		{name: "source assets", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "slot-key-1"}}}, message: "at least one source asset"},
		{name: "slots", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}}, message: "at least one slot"},
		{name: "empty slot id", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{Role: SlotRoleScene, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-1"}}}}, message: "slot id must not be empty"},
		{name: "unknown role", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: "unknown", IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-1"}}}}, message: "unknown slot role"},
		{name: "duplicate idempotency key", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
			{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}},
			{ID: "slot-2", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}},
		}}, message: "duplicate idempotency key"},
		{name: "unknown source reference", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-2"}}}}, message: "source asset reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, ValidatePlan(tt.plan), tt.message)
		})
	}
}
