package preview

import "testing"

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
