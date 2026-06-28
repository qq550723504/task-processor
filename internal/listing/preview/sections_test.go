package preview

import (
	"errors"
	"testing"
)

func TestBuildPlatformSection(t *testing.T) {
	t.Parallel()

	t.Run("skips unselected platform", func(t *testing.T) {
		t.Parallel()

		called := false
		err := BuildPlatformSection("shein", "amazon", true, func() {
			called = true
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

		err := BuildPlatformSection("shein", "shein", false, func() {})
		if err != ErrPlatformUnavailable {
			t.Fatalf("error = %v, want %v", err, ErrPlatformUnavailable)
		}
	})

	t.Run("executes build when available", func(t *testing.T) {
		t.Parallel()

		called := false
		err := BuildPlatformSection("shein", "shein", true, func() {
			called = true
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatal("expected build func to be called")
		}
	})
}

func TestBuildPlatformSectionsRunsBuildersInOrderAndStopsOnError(t *testing.T) {
	t.Parallel()

	var calls []string
	errBoom := errors.New("boom")

	err := BuildPlatformSections([]PlatformSectionBuilder{
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

func TestBuildPlatformSectionsSkipsNilBuilders(t *testing.T) {
	t.Parallel()

	var calls []string
	err := BuildPlatformSections([]PlatformSectionBuilder{
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

func TestPlatformUnavailableError(t *testing.T) {
	t.Parallel()

	if err := PlatformUnavailableError("shein", "amazon"); err != nil {
		t.Fatalf("PlatformUnavailableError(non-selected) = %v, want nil", err)
	}
	if err := PlatformUnavailableError("shein", "shein"); err != ErrPlatformUnavailable {
		t.Fatalf("PlatformUnavailableError(selected) = %v, want %v", err, ErrPlatformUnavailable)
	}
}
