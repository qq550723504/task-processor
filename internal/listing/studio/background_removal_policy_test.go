package studio

import "testing"

func TestSelectBackgroundRemovalTargets(t *testing.T) {
	designs := []BackgroundRemovalDesign{
		{ID: "skip-native", TransparentBackgroundMode: TransparencyModeNative, BackgroundRemovalStatus: BackgroundRemovalStatusNotRequested, ImageURL: "native.png"},
		{ID: "succeeded", TransparentBackgroundMode: TransparencyModeRemoval, BackgroundRemovalStatus: BackgroundRemovalStatusSucceeded, OriginalImageURL: "source-succeeded.png"},
		{ID: "retry", TransparentBackgroundMode: TransparencyModeRemoval, BackgroundRemovalStatus: BackgroundRemovalStatusFailed, OriginalImageURL: "source-retry.png"},
	}

	targets, err := SelectBackgroundRemovalTargets(designs, nil)
	if err != nil {
		t.Fatalf("SelectBackgroundRemovalTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0] != (BackgroundRemovalTarget{DesignIndex: 2, DesignID: "retry", SourceURL: "source-retry.png"}) {
		t.Fatalf("targets = %#v, want retry target", targets)
	}
}

func TestSelectBackgroundRemovalTargetsRequestedIDsUseImageFallbackAndRejectMissingTarget(t *testing.T) {
	designs := []BackgroundRemovalDesign{
		{ID: "design-1", TransparentBackgroundMode: TransparencyModeNone, BackgroundRemovalStatus: BackgroundRemovalStatusNotRequested, ImageURL: "fallback.png"},
	}

	targets, err := SelectBackgroundRemovalTargets(designs, []string{" design-1 ", "design-1"})
	if err != nil {
		t.Fatalf("SelectBackgroundRemovalTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0].SourceURL != "fallback.png" {
		t.Fatalf("targets = %#v, want image fallback", targets)
	}

	if _, err := SelectBackgroundRemovalTargets(designs, []string{"missing"}); err == nil {
		t.Fatal("SelectBackgroundRemovalTargets() error = nil, want missing-target validation")
	}
}

func TestSelectBackgroundRemovalTargetsRejectsPendingOrMissingSource(t *testing.T) {
	for name, design := range map[string]BackgroundRemovalDesign{
		"pending":        {ID: "design-1", TransparentBackgroundMode: TransparencyModeRemoval, BackgroundRemovalStatus: BackgroundRemovalStatusPending, OriginalImageURL: "source.png"},
		"missing source": {ID: "design-1", TransparentBackgroundMode: TransparencyModeRemoval, BackgroundRemovalStatus: BackgroundRemovalStatusFailed},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := SelectBackgroundRemovalTargets([]BackgroundRemovalDesign{design}, nil); err == nil {
				t.Fatal("SelectBackgroundRemovalTargets() error = nil, want validation error")
			}
		})
	}
}
