package canonicalinspect

import (
	"testing"
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
)

func TestDefinitionMetadata(t *testing.T) {
	definition := Definition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("Definition().Validate() error = %v", err)
	}
	if definition.Ref != (commercetool.ToolRef{ID: "product.canonical.inspect", Version: "v1.0.0"}) ||
		definition.Capability != "product.canonical" || definition.Owner != "product.catalog" ||
		definition.Risk != commercetool.RiskRead || definition.SideEffects.Mode != commercetool.SideEffectNone ||
		definition.Idempotency.Mode != commercetool.IdempotencyDeterministic ||
		definition.Retry.Owner != commercetool.RetryOwnerCaller || definition.Usage.Owner != commercetool.UsageOwnerUnmetered ||
		definition.Permission.Permission != authz.PermissionListingKitAdminRead || definition.Timeout.Duration != 3*time.Second {
		t.Fatalf("Definition() metadata = %#v", definition)
	}
}
