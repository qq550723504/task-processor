package platformsection

import (
	"errors"
	"testing"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	if got := Normalize(" SHEIN "); got != "shein" {
		t.Fatalf("Normalize() = %q, want %q", got, "shein")
	}
}

func TestSupportedPlatforms(t *testing.T) {
	t.Parallel()

	want := []string{"amazon", "shein", "temu", "walmart"}
	got := SupportedPlatforms()
	if !equalStrings(got, want) {
		t.Fatalf("SupportedPlatforms() = %#v, want %#v", got, want)
	}

	got[0] = "mutated"
	if got := SupportedPlatforms(); !equalStrings(got, want) {
		t.Fatalf("SupportedPlatforms() after caller mutation = %#v, want %#v", got, want)
	}
}

func TestIsSupported(t *testing.T) {
	t.Parallel()

	if !IsSupported(" SHEIN ") {
		t.Fatal("expected normalized supported platform")
	}
	if IsSupported("ebay") {
		t.Fatal("expected unsupported platform to be rejected")
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
		{name: "normalizes supported platform", input: "  SHEIN ", want: "shein", wantOK: true},
		{name: "empty selection is allowed", input: " ", want: "", wantOK: true},
		{name: "unsupported platform is rejected", input: "ebay", want: "ebay", wantOK: false},
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

func TestShouldBuild(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		selectedPlatform string
		platform         string
		want             bool
	}{
		{name: "all platforms", selectedPlatform: "", platform: "amazon", want: true},
		{name: "selected platform", selectedPlatform: " SHEIN ", platform: "shein", want: true},
		{name: "unselected platform", selectedPlatform: "shein", platform: "amazon", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ShouldBuild(tc.selectedPlatform, tc.platform); got != tc.want {
				t.Fatalf("ShouldBuild(%q, %q) = %v, want %v", tc.selectedPlatform, tc.platform, got, tc.want)
			}
		})
	}
}

func TestIsSelected(t *testing.T) {
	t.Parallel()

	if !IsSelected("temu", " TEMU ") {
		t.Fatal("expected normalized platform match")
	}
	if IsSelected("temu", "walmart") {
		t.Fatal("expected different platform not to match")
	}
}

func TestBuildOne(t *testing.T) {
	t.Parallel()

	t.Run("skips unselected platform", func(t *testing.T) {
		t.Parallel()

		called := false
		err := BuildOne(Section{
			SelectedPlatform: "shein",
			Platform:         "amazon",
			Available:        true,
			Build: func() {
				called = true
			},
			UnavailableError: errors.New("unavailable"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("expected build func to be skipped")
		}
	})

	t.Run("returns unavailable for selected missing platform", func(t *testing.T) {
		t.Parallel()

		errUnavailable := errors.New("unavailable")
		err := BuildOne(Section{
			SelectedPlatform: "shein",
			Platform:         "shein",
			Available:        false,
			Build:            func() {},
			UnavailableError: errUnavailable,
		})
		if err != errUnavailable {
			t.Fatalf("error = %v, want %v", err, errUnavailable)
		}
	})

	t.Run("executes build when available", func(t *testing.T) {
		t.Parallel()

		called := false
		err := BuildOne(Section{
			SelectedPlatform: "shein",
			Platform:         " SHEIN ",
			Available:        true,
			Build: func() {
				called = true
			},
			UnavailableError: errors.New("unavailable"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatal("expected build func to be called")
		}
	})
}

func TestBuildAllRunsBuildersInOrderAndStopsOnError(t *testing.T) {
	t.Parallel()

	var calls []string
	errBoom := errors.New("boom")

	err := BuildAll([]Builder{
		{
			Platform: "amazon",
			Build: func() error {
				calls = append(calls, "amazon")
				return nil
			},
		},
		{
			Platform: "shein",
			Build: func() error {
				calls = append(calls, "shein")
				return errBoom
			},
		},
		{
			Platform: "temu",
			Build: func() error {
				calls = append(calls, "temu")
				return nil
			},
		},
	})

	if err != errBoom {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
	if want := []string{"amazon", "shein"}; !equalStrings(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func TestBuildAllSkipsNilBuilders(t *testing.T) {
	t.Parallel()

	var calls []string
	err := BuildAll([]Builder{
		{Platform: "amazon"},
		{
			Platform: "shein",
			Build: func() error {
				calls = append(calls, "shein")
				return nil
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"shein"}; !equalStrings(calls, want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
