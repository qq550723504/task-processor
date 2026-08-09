package aiidentity

import (
	"context"
	"testing"
)

func TestIdentityContextNormalizesAndRoundTrips(t *testing.T) {
	identity := FromContext(WithIdentity(context.Background(), Identity{TenantID: " tenant-a ", UserID: " user-a "}))
	if identity.TenantID != "tenant-a" || identity.UserID != "user-a" {
		t.Fatalf("identity = %+v", identity)
	}
}
