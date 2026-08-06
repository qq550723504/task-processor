package aicapability

import "testing"

func TestParseRoutingMode(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  RoutingMode
		err   ErrorCategory
	}{
		{value: "", want: RoutingModeLegacy},
		{value: " shadow ", want: RoutingModeShadow},
		{value: "SHADOW", want: RoutingModeShadow},
		{value: "active", want: RoutingModeActive},
		{value: " Active ", want: RoutingModeActive},
		{value: "automatic", err: ErrorInvalidInput},
	} {
		t.Run(tc.value, func(t *testing.T) {
			got, err := ParseRoutingMode(tc.value)
			if tc.err != "" {
				if CategoryOf(err) != tc.err {
					t.Fatalf("CategoryOf(err) = %q, want %q", CategoryOf(err), tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ParseRoutingMode(%q) = %q, %v; want %q, nil", tc.value, got, err, tc.want)
			}
		})
	}
}
