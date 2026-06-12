package preview

import (
	"slices"
	"testing"
)

func TestSupportedPlatforms(t *testing.T) {
	t.Parallel()

	want := []string{"amazon", "shein", "temu", "walmart"}
	if got := SupportedPlatforms(); !slices.Equal(got, want) {
		t.Fatalf("SupportedPlatforms() = %#v, want %#v", got, want)
	}
}

func TestValidateSelectedPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			name:   "normalizes supported platform",
			input:  "  SHEIN ",
			want:   "shein",
			wantOK: true,
		},
		{
			name:   "empty selection is allowed",
			input:  " ",
			want:   "",
			wantOK: true,
		},
		{
			name:   "unsupported platform is rejected",
			input:  "ebay",
			want:   "ebay",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ValidateSelectedPlatform(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("ValidateSelectedPlatform(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestShouldBuildPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		selectedPlatform string
		platform         string
		want             bool
	}{
		{
			name:             "empty selection builds all platforms",
			selectedPlatform: "",
			platform:         "shein",
			want:             true,
		},
		{
			name:             "matching selection builds platform",
			selectedPlatform: "shein",
			platform:         "shein",
			want:             true,
		},
		{
			name:             "different selection skips platform",
			selectedPlatform: "amazon",
			platform:         "shein",
			want:             false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ShouldBuildPlatform(tt.selectedPlatform, tt.platform); got != tt.want {
				t.Fatalf("ShouldBuildPlatform(%q, %q) = %v, want %v", tt.selectedPlatform, tt.platform, got, tt.want)
			}
		})
	}
}

func TestIsSelectedPlatform(t *testing.T) {
	t.Parallel()

	if !IsSelectedPlatform("temu", " TEMU ") {
		t.Fatal("expected matching platform to be selected")
	}
	if IsSelectedPlatform("temu", "walmart") {
		t.Fatal("expected different platform to be unselected")
	}
}
