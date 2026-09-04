package imageagent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProjectionSnapshotAcceptsOnlyValidatedDurableV3Candidates(t *testing.T) {
	const (
		tenantID = "tenant-a"
		runID    = "run-a"
	)
	sha := strings.Repeat("a", 64)
	plan := Plan{Revision: 1, IdempotencyKey: "plan-key", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "scene-1", Role: SlotRoleMain, IdempotencyKey: "scene-key", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending}}}
	run := Run{ID: runID, TenantID: tenantID, UserID: "user-a", ActivePlanRevision: 1}
	snapshot := RunProjection{
		Run:               run,
		Plan:              plan,
		ProjectionVersion: 1,
		LastEventID:       1,
		Slots: []SlotProjection{{
			Slot:    Slot{ID: "scene-1", Role: SlotRoleMain, Status: SlotStatusAccepted},
			Attempt: 1,
			Candidates: []AssetCandidate{{
				AssetID:       "candidate-1",
				SourceAssetID: "source-1",
				DurableAsset:  DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/fc95297aa4f56781f0decb7d4bf59b1447f09b3611039b80188b1c6beb03ee6a/run-a/1/scene-1/1/0-" + sha + ".png", SHA256: sha},
			}},
		}},
	}

	require.NoError(t, ValidateProjectionSnapshot(ScopeForRun(run), snapshot))

	snapshot.Slots[0].Candidates[0].URL = "https://cdn.example.test/legacy.png"
	require.ErrorIs(t, ValidateProjectionSnapshot(ScopeForRun(run), snapshot), ErrValidation)
}

func TestSlotProjectionJSONPreservesV3DurableIdentityWithoutChangingV2CandidateWire(t *testing.T) {
	sha := strings.Repeat("a", 64)
	v3 := SlotProjection{Candidates: []AssetCandidate{{
		AssetID: "candidate-1", SourceAssetID: "source-1",
		DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/fc95297aa4f56781f0decb7d4bf59b1447f09b3611039b80188b1c6beb03ee6a/run-a/1/scene-1/1/0-" + sha + ".png", SHA256: sha},
	}}}
	encoded, err := json.Marshal(v3)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"DurableAsset"`)
	require.NotContains(t, string(encoded), `"URL"`)
	require.NotContains(t, string(encoded), `"Metadata"`)

	var decoded SlotProjection
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, v3, decoded)

	v2, err := json.Marshal(SlotProjection{Candidates: []AssetCandidate{{AssetID: "candidate-1", URL: "https://cdn.example.test/legacy.png", SourceAssetID: "source-1", Metadata: map[string]string{"legacy": "kept"}}}})
	require.NoError(t, err)
	require.NotContains(t, string(v2), `"DurableAsset"`)
}

func TestNormalizeRecoverableEffectsDeduplicatesStableEntriesAndRejectsConflicts(t *testing.T) {
	normalized, err := NormalizeRecoverableEffects([]RecoverableEffect{
		{SlotID: "slot-1", Attempt: 1, Code: "recovery_requested"},
		{SlotID: "slot-1", Attempt: 1, Code: "recovery_requested"},
		{SlotID: "slot-2", Attempt: 2, Code: "recovery_start_failed"},
	})
	require.NoError(t, err)
	require.Equal(t, []RecoverableEffect{
		{SlotID: "slot-1", Attempt: 1, Code: "recovery_requested"},
		{SlotID: "slot-2", Attempt: 2, Code: "recovery_start_failed"},
	}, normalized)

	_, err = NormalizeRecoverableEffects([]RecoverableEffect{
		{SlotID: "slot-1", Attempt: 1, Code: "recovery_requested"},
		{SlotID: "slot-1", Attempt: 1, Code: "recovery_start_failed"},
	})
	require.ErrorIs(t, err, ErrRevisionConflict)
}

func TestNormalizeRecoverableEffectsAcceptsReviewerTransportBlocks(t *testing.T) {
	normalized, err := NormalizeRecoverableEffects([]RecoverableEffect{{
		SlotID: "slot-1", Attempt: 1, Code: SlotReviewTransportRequiredCode,
	}})

	require.NoError(t, err)
	require.Equal(t, []RecoverableEffect{{
		SlotID: "slot-1", Attempt: 1, Code: SlotReviewTransportRequiredCode,
	}}, normalized)
}

func TestValidateProjectionSnapshotRejectsRecoverableEffectWithoutMatchingBlockedSlot(t *testing.T) {
	run := Run{ID: "run-a", TenantID: "tenant-a", UserID: "user-a", ActivePlanRevision: 1, Status: RunStatusBlocked}
	plan := Plan{Revision: 1, IdempotencyKey: "plan-key", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "scene-1", Role: SlotRoleMain, IdempotencyKey: "scene-key", SourceAssetIDs: []string{"source-1"}, Status: SlotStatusPending}}}
	snapshot := RunProjection{
		Run:               run,
		Plan:              plan,
		ProjectionVersion: 1,
		LastEventID:       1,
		Slots:             []SlotProjection{{Slot: Slot{ID: "scene-1", Role: SlotRoleMain, Status: SlotStatusBlocked}, Attempt: 1, ErrorCode: "recovery_requested"}},
		RecoverableEffects: []RecoverableEffect{
			{SlotID: "scene-1", Attempt: 2, Code: "recovery_requested"},
		},
	}

	require.ErrorIs(t, ValidateProjectionSnapshot(ScopeForRun(run), snapshot), ErrRevisionConflict)
}
