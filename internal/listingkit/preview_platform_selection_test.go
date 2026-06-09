package listingkit

import "testing"

func TestShouldBuildPreviewPlatform(t *testing.T) {
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

			if got := shouldBuildPreviewPlatform(tt.selectedPlatform, tt.platform); got != tt.want {
				t.Fatalf("shouldBuildPreviewPlatform(%q, %q) = %v, want %v", tt.selectedPlatform, tt.platform, got, tt.want)
			}
		})
	}
}

func TestIsSelectedPreviewPlatform(t *testing.T) {
	t.Parallel()

	if !isSelectedPreviewPlatform("temu", "temu") {
		t.Fatal("expected matching platform to be selected")
	}
	if isSelectedPreviewPlatform("temu", "walmart") {
		t.Fatal("expected different platform to be unselected")
	}
}

func TestPreviewPlatformBuilders(t *testing.T) {
	t.Parallel()

	builders := previewPlatformBuilders()
	want := []string{"amazon", "shein", "temu", "walmart"}
	if len(builders) != len(want) {
		t.Fatalf("previewPlatformBuilders() length = %d, want %d", len(builders), len(want))
	}
	for i, builder := range builders {
		if got := builder.platform(); got != want[i] {
			t.Fatalf("previewPlatformBuilders()[%d].platform() = %q, want %q", i, got, want[i])
		}
	}
}

func TestBuildListingKitPreviewReturnsUnavailableForMissingSelectedPlatformPayload(t *testing.T) {
	t.Parallel()

	_, err := buildListingKitPreview(&Task{
		ID:     "task-preview-missing-platform",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID:    "task-preview-missing-platform",
			Status:    "completed",
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
