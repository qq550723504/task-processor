package listingkit

import "testing"

func TestOwnerScopeEnabledByDefault(t *testing.T) {
	if !OwnerScopeEnabled() {
		t.Fatal("owner scope must be enabled before HTTP bootstrap")
	}
}
