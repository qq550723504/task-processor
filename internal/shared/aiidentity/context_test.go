package aiidentity

import (
	"context"
	"testing"
)

func TestIdentityContextNormalizesAndRoundTrips(t *testing.T) {
	identity := FromContext(WithIdentity(context.Background(), Identity{
		TenantID:       " tenant-a ",
		UserID:         " user-a ",
		BusinessTaskID: " task-a ",
		TraceID:        " trace-a ",
	}))
	if identity.TenantID != "tenant-a" || identity.UserID != "user-a" || identity.BusinessTaskID != "task-a" || identity.TraceID != "trace-a" {
		t.Fatalf("identity = %+v", identity)
	}
}
