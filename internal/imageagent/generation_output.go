package imageagent

import "context"

// GenerationOutputRecovery performs only bounded GET/materialization of an
// already-observed success, after live execution and catalog authorization.
// It is separate from the authorization-independent financial finalizer.
type GenerationOutputRecovery func(context.Context, SlotExecutionInput, GenerationFact) (SlotGeneratedOutput, error)

type GenerationStagingRecoveryRepository interface {
	RecoverGenerationStaging(context.Context, SlotEffectV3Reservation, GenerationIntent, string, StagingManifest) (SlotEffectV3Attempt, error)
}
