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
	plan := Plan{Revision: 1, IdempotencyKey: "plan-key", SourceAssetIDs: []string{"source-1"}, Slots: []Slot{{ID: "scene-1", Role: SlotRoleScene, IdempotencyKey: "scene-key", SourceAssetIDs: []string{"source-1"}}}}
	run := Run{ID: runID, TenantID: tenantID, UserID: "user-a", ActivePlanRevision: 1}
	snapshot := RunProjection{
		Run:               run,
		Plan:              plan,
		ProjectionVersion: 1,
		LastEventID:       1,
		Slots: []SlotProjection{{
			Slot:    Slot{ID: "scene-1", Role: SlotRoleScene, Status: SlotStatusAccepted},
			Attempt: 1,
			Candidates: []AssetCandidate{{
				AssetID:       "candidate-1",
				SourceAssetID: "source-1",
				DurableAsset:  DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-a/1/scene-1/1/0-" + sha + ".png", SHA256: sha},
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
		DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-a/1/scene-1/1/0-" + sha + ".png", SHA256: sha},
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
