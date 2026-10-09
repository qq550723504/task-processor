package imageagent

import (
	"context"
	"errors"
)

// ErrInvalidGeneratedOutput proves a fetched success artifact is permanently
// unusable. Identity, network and unavailable-artifact errors do not qualify.
var ErrInvalidGeneratedOutput = errors.New(InvalidGeneratedOutputCode)

// GenerationOutputRecovery performs only bounded GET/materialization of an
// already-observed success, after live execution and catalog authorization.
// It is separate from the authorization-independent financial finalizer.
type GenerationOutputRecovery func(context.Context, SlotExecutionInput, GenerationFact) (SlotGeneratedOutput, error)

type GenerationStagingRecoveryRepository interface {
	RecoverGenerationStaging(context.Context, SlotEffectV3Reservation, GenerationIntent, string, StagingManifest) (SlotEffectV3Attempt, error)
}
