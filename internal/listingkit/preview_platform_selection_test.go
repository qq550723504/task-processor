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

func TestValidateSelectedPreviewPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "normalizes supported platform",
			input: "  SHEIN ",
			want:  "shein",
		},
		{
			name:    "rejects unsupported platform",
			input:   "ebay",
			wantErr: ErrUnsupportedPreviewPlatform,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateSelectedPreviewPlatform(tt.input)
			if err != tt.wantErr {
				t.Fatalf("validateSelectedPreviewPlatform(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("validateSelectedPreviewPlatform(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildPendingPreviewHeader(t *testing.T) {
	t.Parallel()

	header := buildPendingPreviewHeader(&Task{Status: TaskStatusProcessing})
	if header == nil {
		t.Fatal("expected header")
	}
	if header.StatusMessage != "任务处理中，预览结果尚未准备完成" {
		t.Fatalf("status message = %q", header.StatusMessage)
	}
}

func TestBuildPreviewPlatformSection(t *testing.T) {
	t.Parallel()

	t.Run("skips unselected platform", func(t *testing.T) {
		t.Parallel()

		called := false
		err := buildPreviewPlatformSection("shein", "amazon", true, func() {
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

		err := buildPreviewPlatformSection("shein", "shein", false, func() {})
		if err != ErrPreviewPlatformUnavailable {
			t.Fatalf("error = %v, want %v", err, ErrPreviewPlatformUnavailable)
		}
	})

	t.Run("executes build when available", func(t *testing.T) {
		t.Parallel()

		called := false
		err := buildPreviewPlatformSection("shein", "shein", true, func() {
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
