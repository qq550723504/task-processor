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

func TestDescriptorOrganizationAccessPolicyValuesAndLegacyDefault(t *testing.T) {
	tests := []struct {
		policy OrganizationAccessPolicy
		want   string
	}{
		{policy: OrganizationAccessPolicyNone, want: "none"},
		{policy: OrganizationAccessPolicyContextRead, want: "context_read"},
		{policy: OrganizationAccessPolicyCachedRead, want: "cached_read"},
		{policy: OrganizationAccessPolicyLiveWrite, want: "live_write"},
		{policy: OrganizationAccessPolicyLiveSwitch, want: "live_switch"},
	}
	for _, tt := range tests {
		if got := string(tt.policy); got != tt.want {
			t.Fatalf("policy = %q, want %q", got, tt.want)
		}
	}

	route := Descriptor{Method: http.MethodGet, Path: "/legacy"}
	if route.OrganizationAccessPolicy != "" {
		t.Fatalf("legacy OrganizationAccessPolicy = %q, want blank zero value", route.OrganizationAccessPolicy)
	}
}
