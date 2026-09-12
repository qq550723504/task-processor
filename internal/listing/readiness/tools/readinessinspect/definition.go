package readinessinspect

import (
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"time"
)

func Definition() commercetool.Definition {
	return commercetool.Definition{
		Ref:        commercetool.ToolRef{ID: "product.readiness.inspect", Version: "v1.0.0"},
		Capability: "product.readiness", Owner: "listing.readiness",
		Description: "Diagnose exact Product input readiness; marketplace rules remain not evaluated. This does not authorize publication.",
		InputSchema: InputSchema(), OutputSchema: OutputSchema(), Risk: commercetool.RiskRead,
		Permission:  commercetool.PermissionRequirement{Permission: authz.PermissionListingKitAdminRead},
		SideEffects: commercetool.SideEffectPolicy{Mode: commercetool.SideEffectNone},
		Idempotency: commercetool.IdempotencyPolicy{Mode: commercetool.IdempotencyDeterministic},
		Timeout:     commercetool.TimeoutPolicy{Duration: 3 * time.Second},
		Retry:       commercetool.RetryPolicy{Owner: commercetool.RetryOwnerCaller},
		Usage:       commercetool.UsagePolicy{Owner: commercetool.UsageOwnerUnmetered},
	}
}
