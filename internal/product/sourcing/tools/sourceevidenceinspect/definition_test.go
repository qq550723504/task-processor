package sourceevidenceinspect

import (
	"task-processor/internal/authz"
	"task-processor/internal/commercetool"
	"testing"
	"time"
)

func TestDefinitionKeepsExistingAuthorizationAndNoEffects(t *testing.T) {
	d := Definition()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if d.Ref != (commercetool.ToolRef{ID: "product.source-evidence.inspect", Version: "v1.0.0"}) || d.Owner != "product.sourcing" || d.Permission.Permission != authz.PermissionProductSourcingWrite ||
		d.Risk != commercetool.RiskRead || d.SideEffects.Mode != commercetool.SideEffectNone || d.Idempotency.Mode != commercetool.IdempotencyDeterministic || d.Timeout.Duration != 3*time.Second || d.Usage.Owner != commercetool.UsageOwnerUnmetered || d.Retry.Owner != commercetool.RetryOwnerCaller {
		t.Fatalf("definition=%#v", d)
	}
}
