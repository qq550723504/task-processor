package httproute

import (
	"net/http"
	"testing"
)

func TestDescriptorAuthPolicyDefaultsToUnspecifiedForLegacyRoutes(t *testing.T) {
	route := Descriptor{Method: http.MethodGet, Path: "/health"}
	if route.AuthPolicy != AuthPolicyUnspecified {
		t.Fatalf("AuthPolicy = %v, want unspecified", route.AuthPolicy)
	}
}
