package imageagent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResultDigestV2MatchesCommittedHistoricalValue(t *testing.T) {
	plan := Plan{Revision: 1, Slots: []Slot{{ID: "main", Role: SlotRoleMain}, {ID: "scene", Role: SlotRoleScene}}}
	sceneCandidates := make([]AssetCandidate, 11)
	for index := range sceneCandidates {
		sceneCandidates[index] = AssetCandidate{AssetID: fmt.Sprintf("candidate-scene-%02d", index+1)}
	}
	slots := []SlotProjection{
		{Slot: Slot{ID: "main", Role: SlotRoleMain, Status: SlotStatusAccepted}, Candidates: []AssetCandidate{{AssetID: "candidate-main"}}},
		{Slot: Slot{ID: "scene", Role: SlotRoleScene, Status: SlotStatusAccepted}, Candidates: sceneCandidates},
	}

	digest, err := ResultDigestV2(plan, slots)
	require.NoError(t, err)
	require.Equal(t, "9e5c8dba27d1224662e48945bf1456d7c339f541250228b068abafe8a944c0e6", digest)
}

func TestResultDigestV3ChangesWithPlanRoleObjectKeyHashOrOrder(t *testing.T) {
	plan, slots := digestV3Fixture()
	baseline, err := ResultDigestV3(plan, slots)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*Plan, []SlotProjection)
	}{
		{name: "plan revision", mutate: func(plan *Plan, _ []SlotProjection) { plan.Revision++ }},
		{name: "role", mutate: func(plan *Plan, slots []SlotProjection) {
			plan.Slots[1].Role, slots[1].Slot.Role = SlotRoleDetail, SlotRoleDetail
		}},
		{name: "object key", mutate: func(_ *Plan, slots []SlotProjection) {
			slots[1].Candidates[0].DurableAsset.ObjectKey = "image-agent/public/tenant-a/run-1/1/scene/1/0-" + strings.Repeat("b", 64) + ".png"
		}},
		{name: "hash", mutate: func(_ *Plan, slots []SlotProjection) {
			slots[1].Candidates[0].DurableAsset.SHA256 = strings.Repeat("c", 64)
		}},
		{name: "candidate order", mutate: func(_ *Plan, slots []SlotProjection) {
			slots[1].Candidates[0], slots[1].Candidates[1] = slots[1].Candidates[1], slots[1].Candidates[0]
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidatePlan, candidateSlots := digestV3Fixture()
			tt.mutate(&candidatePlan, candidateSlots)
			digest, err := ResultDigestV3(candidatePlan, candidateSlots)
			require.NoError(t, err)
			require.NotEqual(t, baseline, digest)
		})
	}
}

func digestV3Fixture() (Plan, []SlotProjection) {
	shaA := strings.Repeat("a", 64)
	shaB := strings.Repeat("b", 64)
	plan := Plan{Revision: 1, Slots: []Slot{{ID: "main", Role: SlotRoleMain}, {ID: "scene", Role: SlotRoleScene}}}
	slots := []SlotProjection{
		{Slot: Slot{ID: "main", Role: SlotRoleMain, Status: SlotStatusAccepted}, Candidates: []AssetCandidate{{AssetID: "candidate-main", DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/main/1/0-" + shaA + ".png", SHA256: shaA}}}},
		{Slot: Slot{ID: "scene", Role: SlotRoleScene, Status: SlotStatusAccepted}, Candidates: []AssetCandidate{
			{AssetID: "candidate-scene-01", DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/scene/1/0-" + shaA + ".png", SHA256: shaA}},
			{AssetID: "candidate-scene-02", DurableAsset: DurableAssetIdentity{ObjectKey: "image-agent/public/tenant-a/run-1/1/scene/1/1-" + shaB + ".png", SHA256: shaB}},
		}},
	}
	return plan, slots
}
