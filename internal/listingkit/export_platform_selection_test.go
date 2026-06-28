package listingkit

import "testing"

func TestExportPlatformBuilders(t *testing.T) {
	t.Parallel()

	builders := exportPlatformBuilders()
	want := []string{"amazon", "shein", "temu", "walmart"}
	if len(builders) != len(want) {
		t.Fatalf("exportPlatformBuilders() length = %d, want %d", len(builders), len(want))
	}
	for i, builder := range builders {
		if got := builder.Platform(); got != want[i] {
			t.Fatalf("exportPlatformBuilders()[%d].Platform() = %q, want %q", i, got, want[i])
		}
	}
}

func TestApplyExportPlatformSection(t *testing.T) {
	t.Parallel()

	t.Run("skips unselected platform", func(t *testing.T) {
		t.Parallel()

		called := false
		err := applyExportPlatformSection("shein", "amazon", true, func() {
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

		err := applyExportPlatformSection("shein", "shein", false, func() {})
		if err != ErrPreviewPlatformUnavailable {
			t.Fatalf("error = %v, want %v", err, ErrPreviewPlatformUnavailable)
		}
	})

	t.Run("executes build when available", func(t *testing.T) {
		t.Parallel()

		called := false
		err := applyExportPlatformSection("shein", "shein", true, func() {
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

func TestBuildListingKitExportReturnsUnavailableForMissingSelectedPlatformPayload(t *testing.T) {
	t.Parallel()

	_, err := buildListingKitExport(&Task{
		ID: "task-export-missing-platform",
		Result: &ListingKitResult{
			Platforms: []string{"amazon", "shein"},
		},
	}, "shein")
	if err == nil {
		t.Fatal("expected unavailable platform error")
	}
	if err != ErrPreviewPlatformUnavailable {
		t.Fatalf("error = %v, want %v", err, ErrPreviewPlatformUnavailable)
	}
}
