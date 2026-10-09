package imageagent

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImageSetGenerationInputBindsExactSourcesPromptAndConfiguration(t *testing.T) {
	plan := setPlanFixture(t)
	input := SlotExecutionInput{RunID: "run", TenantID: "org", UserID: "actor", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt", Slot: plan.Slots[0], ImageSet: plan.Set, TargetPlatform: "product"}
	first, err := ImageSlotGenerationInputDigest(input)
	require.NoError(t, err)
	for _, mode := range []string{"prompt", "source", "order", "configuration", "target"} {
		t.Run(mode, func(t *testing.T) {
			changed := input
			changed.Slot.Recipe = CloneImageSlotRecipe(input.Slot.Recipe)
			changed.ImageSet = CloneImageSetPlan(input.ImageSet)
			switch mode {
			case "prompt":
				changed.Slot.Recipe.Prompt = "different approved purpose"
			case "source":
				changed.Slot.Recipe.References[0].SHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			case "order":
				changed.Slot.Recipe.Placement.Order++
			case "configuration":
				changed.ImageSet.Configuration.Digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			case "target":
				changed.ImageSet.Target.StoreID = "store-changed"
			}
			digest, err := ImageSlotGenerationInputDigest(changed)
			if err == nil {
				require.NotEqual(t, first, digest)
			}
		})
	}
	intent := generationTestIntent()
	old, err := NewGenerationFact(intent)
	require.NoError(t, err)
	require.Equal(t, 1, old.Version)
	intent.InputDigest = first
	intent.InputProtocol = ImageSetSchema
	next, err := NewGenerationFact(intent)
	require.NoError(t, err)
	require.Equal(t, 2, next.Version)
	require.NotEqual(t, old.Fingerprint, next.Fingerprint)
	require.NoError(t, next.Validate())
	intent.InputProtocol = ""
	_, err = NewGenerationFact(intent)
	require.Error(t, err)
}
