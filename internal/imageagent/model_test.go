package imageagent

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaxActionIDFitsLongestProjectionCommitIdentity(t *testing.T) {
	actionID := strings.Repeat("a", MaxActionIDLength)
	require.NoError(t, ValidateActionID(actionID))
	commitID := fmt.Sprintf("command:%s:attempt:%d:%s", actionID, int64(math.MaxInt64), "approval.persist_complete")
	require.LessOrEqual(t, len(commitID), 192)
}

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

func TestNormalizeAssetCatalogRejectsPlainHTTPSourceImages(t *testing.T) {
	_, err := NormalizeAssetCatalog(AssetCatalog{Assets: []AuthorizedAsset{{
		ID: "source-1", Type: AuthorizedAssetSource, URL: "http://images.example/source.png",
	}}})
	require.ErrorContains(t, err, "public https url is required")
}

func TestNormalizeAssetCatalogBindsCanonicalProductContextToManifest(t *testing.T) {
	assets := []AuthorizedAsset{{ID: "source-1", Type: AuthorizedAssetSource, URL: "https://source.example/source.png"}}
	catalog, err := NormalizeAssetCatalog(AssetCatalog{Assets: assets, ProductContext: ProductContextRef{
		ProductID: " task-1 ", Title: " Travel Bottle ", ProductType: " Bottles ",
		Attributes: map[string]string{" Material ": " Steel ", "ignored": " "},
	}})
	require.NoError(t, err)
	require.Equal(t, ProductContextRef{ProductID: "task-1", Title: "Travel Bottle", ProductType: "Bottles", Attributes: map[string]string{"Material": "Steel"}}, catalog.ProductContext)
	require.Contains(t, catalog.Manifest.Hash, "catalog-v2:")

	changed := catalog
	changed.Manifest.Hash = ""
	changed.ProductContext.Title = "Changed title"
	changed, err = NormalizeAssetCatalog(changed)
	require.NoError(t, err)
	require.NotEqual(t, catalog.Manifest.Hash, changed.Manifest.Hash)

	legacy, err := NormalizeAssetCatalog(AssetCatalog{Assets: assets})
	require.NoError(t, err)
	require.Equal(t, CatalogHash(legacy.Assets), legacy.Manifest.Hash)
}

func TestValidateSubmittedPlanAgainstCatalogRequiresReliableDimensionsForSizeSlotFirstSource(t *testing.T) {
	plan := Plan{
		SourceAssetIDs: []string{"source-unmeasured", "source-measured"},
		Slots: []Slot{{
			ID: "size-1", Role: SlotRoleSize,
			SourceAssetIDs: []string{"source-unmeasured", "source-measured"},
		}},
	}
	catalog := AssetCatalog{Assets: []AuthorizedAsset{
		{ID: "source-unmeasured", Type: AuthorizedAssetSource},
		{ID: "source-measured", Type: AuthorizedAssetSource, Width: 1200, Height: 900},
	}}

	require.NoError(t, ValidatePlanAgainstCatalog(plan, catalog), "historical workflow snapshots retain the pre-ingress compatibility contract")
	require.ErrorContains(t, ValidateSubmittedPlanAgainstCatalog(plan, catalog), "reliable dimensions")
	plan.Slots[0].SourceAssetIDs = []string{"source-measured", "source-unmeasured"}
	require.NoError(t, ValidateSubmittedPlanAgainstCatalog(plan, catalog), "the executor deterministically uses the first selected source")
}

func TestValidatePlanAllowsMoreThanTenIndependentSlots(t *testing.T) {
	slots := make([]Slot, 11)
	for i := range slots {
		role := SlotRoleScene
		if i == 0 {
			role = SlotRoleMain
		}
		slots[i] = Slot{ID: fmt.Sprintf("slot-%d", i), Role: role, IdempotencyKey: fmt.Sprintf("slot-key-%d", i), SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending}
	}
	err := ValidatePlan(Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: slots})
	require.NoError(t, err)
}

func TestValidatePlanRequiresExactlyOneMainSlot(t *testing.T) {
	base := Plan{
		Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"},
		Slots: []Slot{{ID: "main-1", Role: SlotRoleMain, IdempotencyKey: "main-key-1", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending}},
	}
	withoutMain := base
	withoutMain.Slots = []Slot{{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key-1", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending}}
	withTwoMain := base
	withTwoMain.Slots = append(append([]Slot(nil), base.Slots...), Slot{ID: "main-2", Role: SlotRoleMain, IdempotencyKey: "main-key-2", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending})

	for name, plan := range map[string]Plan{"missing": withoutMain, "duplicate": withTwoMain} {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, ValidatePlan(plan), "exactly one main slot")
		})
	}
}

func TestValidatePlanRejectsDuplicateSlotIDs(t *testing.T) {
	plan := Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
		{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key-1", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending},
		{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key-2", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending},
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

func TestProviderAndStagingUnknownPermitNewAttempt(t *testing.T) {
	for _, code := range []string{SlotProviderOutcomeUnknownCode, SlotStagingOutcomeUnknownCode} {
		run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: code, SlotID: "scene-2"}}
		require.True(t, BlockAllowsAction(run.Block, ActionRetrySlot), code)
	}
}

func TestPublicationUnknownDoesNotPermitBlindRetry(t *testing.T) {
	run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: SlotPublicationOutcomeUnknownCode, SlotID: "scene-2"}}
	require.False(t, BlockAllowsAction(run.Block, ActionRetrySlot))
	require.Equal(t, []Action{ActionEditPlan, ActionCancel}, AllowedActions(run))
}

func TestAllowedActionsForInvalidOrUnknownV3PolicyFailClosed(t *testing.T) {
	for _, code := range []string{
		"slot_effect_phase_invalid",
		"slot_effect_policy_invalid",
		"slot_effect_future_policy_invalid",
	} {
		run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: code, SlotID: "scene-2"}}
		require.Equal(t, []Action{ActionCancel}, AllowedActions(run), code)
		require.False(t, BlockAllowsAction(run.Block, ActionRetrySlot), code)
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

func TestValidatePlanBoundsPersistedIdempotencyKeys(t *testing.T) {
	plan := Plan{
		Revision: 1, IdempotencyKey: strings.Repeat("p", MaxIdempotencyKeyLength), SourceAssetIDs: []string{"source-1"},
		Slots: []Slot{{ID: "main-1", Role: SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: strings.Repeat("s", MaxIdempotencyKeyLength), Status: SlotStatusPending}},
	}
	require.NoError(t, ValidateSubmittedPlan(plan))

	tooLongPlan := plan
	tooLongPlan.IdempotencyKey += "p"
	require.ErrorContains(t, ValidateSubmittedPlan(tooLongPlan), "plan idempotency key")

	tooLongSlot := plan
	tooLongSlot.Slots = append([]Slot(nil), plan.Slots...)
	tooLongSlot.Slots[0].IdempotencyKey += "s"
	require.ErrorContains(t, ValidateSubmittedPlan(tooLongSlot), "slot idempotency key")
}

func TestValidateSubmittedPlanRequiresExplicitPendingSlotStatus(t *testing.T) {
	plan := Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "main-1", Role: SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1"}}}
	require.NoError(t, ValidatePlan(plan), "legacy histories may omit the formerly implicit pending status")
	require.ErrorContains(t, ValidateSubmittedPlan(plan), "slot status must be pending")
	plan.Slots[0].Status = SlotStatusPending
	require.NoError(t, ValidateSubmittedPlan(plan))
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
		{name: "slot source assets", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "main-1", Role: SlotRoleMain, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"", "  "}}}}, message: "slot requires at least one source asset"},
		{name: "slot status", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "main-1", Role: SlotRoleMain, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusAccepted}}}, message: "slot status must be pending"},
		{name: "duplicate idempotency key", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
			{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending},
			{ID: "slot-2", Role: SlotRoleScene, IdempotencyKey: "same", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending},
		}}, message: "duplicate idempotency key"},
		{name: "unknown source reference", plan: Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "slot-1", Role: SlotRoleScene, IdempotencyKey: "slot-key-1", SourceAssetIDs: []string{"source-2"}}}}, message: "source asset reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, ValidatePlan(tt.plan), tt.message)
		})
	}
}
