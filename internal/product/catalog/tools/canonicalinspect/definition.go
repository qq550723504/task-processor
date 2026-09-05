package canonicalinspect

import (
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
)

func Definition() commercetool.Definition {
	return commercetool.Definition{
		Ref:          commercetool.ToolRef{ID: "product.canonical.inspect", Version: "v1.0.0"},
		Capability:   "product.canonical",
		Owner:        "product.catalog",
		Description:  "Inspect the authorized immutable canonical product snapshot and source lineage.",
		InputSchema:  InputSchema(),
		OutputSchema: OutputSchema(),
		Risk:         commercetool.RiskRead,
		Permission:   commercetool.PermissionRequirement{Permission: authz.PermissionListingKitAdminRead},
		SideEffects:  commercetool.SideEffectPolicy{Mode: commercetool.SideEffectNone},
		Idempotency:  commercetool.IdempotencyPolicy{Mode: commercetool.IdempotencyDeterministic},
		Timeout:      commercetool.TimeoutPolicy{Duration: 3 * time.Second},
		Retry:        commercetool.RetryPolicy{Owner: commercetool.RetryOwnerCaller},
		Usage:        commercetool.UsagePolicy{Owner: commercetool.UsageOwnerUnmetered},
	}
}
