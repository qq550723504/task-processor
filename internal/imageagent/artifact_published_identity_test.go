package imageagent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePublishedAssetIdentityForSlotBindsDeterministicPublicKey(t *testing.T) {
	hash := strings.Repeat("a", 64)
	input := SlotExecutionInput{
		RunID: "run-1", TenantID: "tenant-a", UserID: "user-a", PlanRevision: 3,
		Slot: Slot{ID: "scene-1"}, Attempt: 2,
	}
	valid := DurableAssetIdentity{
		ObjectKey: "image-agent/public/tenant-a/run-1/3/scene-1/2/4-" + hash + ".png",
		SHA256:    hash,
	}
	require.NoError(t, ValidatePublishedAssetIdentityForSlot(input, valid, 4))

	for _, test := range []struct {
		name   string
		mutate func(*DurableAssetIdentity)
	}{
		{name: "staging prefix", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "image-agent/public/", "image-agent/staging/", 1)
		}},
		{name: "tenant", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/tenant-a/", "/tenant-b/", 1)
		}},
		{name: "run", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/run-1/", "/run-b/", 1)
		}},
		{name: "revision", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/3/scene-1/", "/4/scene-1/", 1)
		}},
		{name: "slot", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/scene-1/", "/scene-2/", 1)
		}},
		{name: "attempt", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/2/4-", "/3/4-", 1)
		}},
		{name: "index", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, "/4-", "/5-", 1)
		}},
		{name: "hash", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.Replace(identity.ObjectKey, hash+".png", strings.Repeat("b", 64)+".png", 1)
		}},
		{name: "extension", mutate: func(identity *DurableAssetIdentity) {
			identity.ObjectKey = strings.TrimSuffix(identity.ObjectKey, ".png") + ".gif"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity := valid
			test.mutate(&identity)
			require.ErrorIs(t, ValidatePublishedAssetIdentityForSlot(input, identity, 4), ErrValidation)
		})
	}
}
