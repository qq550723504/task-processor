package sourceevidenceinspect

import (
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"time"
)

func Definition() commercetool.Definition {
	return commercetool.Definition{
		Ref:        commercetool.ToolRef{ID: "product.source-evidence.inspect", Version: "v1.0.0"},
		Capability: "product.source-evidence", Owner: "product.sourcing",
		Description: "Inspect structured source evidence for one exact Catalog publication; raw text and source URLs are not returned.",
		InputSchema: InputSchema(), OutputSchema: OutputSchema(), Risk: commercetool.RiskRead,
		Permission:  commercetool.PermissionRequirement{Permission: authz.PermissionProductSourcingWrite},
		SideEffects: commercetool.SideEffectPolicy{Mode: commercetool.SideEffectNone},
		Idempotency: commercetool.IdempotencyPolicy{Mode: commercetool.IdempotencyDeterministic},
		Timeout:     commercetool.TimeoutPolicy{Duration: 3 * time.Second}, Retry: commercetool.RetryPolicy{Owner: commercetool.RetryOwnerCaller},
		Usage: commercetool.UsagePolicy{Owner: commercetool.UsageOwnerUnmetered},
	}
}
