package studio

import "testing"

func TestNormalizeTransparencyModePrefersExplicitModeAndSupportsLegacyFlag(t *testing.T) {
	tests := []struct {
		name   string
		mode   string
		legacy *bool
		want   TransparencyMode
	}{
		{name: "explicit removal", mode: "removal", legacy: boolPtr(true), want: TransparencyModeRemoval},
		{name: "explicit native", mode: "native", want: TransparencyModeNative},
		{name: "legacy native", legacy: boolPtr(true), want: TransparencyModeNative},
		{name: "legacy none", legacy: boolPtr(false), want: TransparencyModeNone},
		{name: "invalid mode", mode: "mask", legacy: boolPtr(true), want: TransparencyModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTransparencyMode(tt.mode, tt.legacy); got != tt.want {
				t.Fatalf("NormalizeTransparencyMode(%q, %v) = %q, want %q", tt.mode, tt.legacy, got, tt.want)
			}
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}
