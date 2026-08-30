package studio

import "testing"

func TestFindManualBackgroundRemovalDesign(t *testing.T) {
	designs := []BackgroundRemovalDesign{
		{ID: "design-1", BackgroundRemovalStatus: BackgroundRemovalStatusFailed},
		{ID: "design-2", BackgroundRemovalStatus: BackgroundRemovalStatusNotRequested},
	}

	index, found, err := FindManualBackgroundRemovalDesign(designs, " design-2 ")
	if err != nil || !found || index != 1 {
		t.Fatalf("FindManualBackgroundRemovalDesign() = (%d, %t, %v), want (1, true, nil)", index, found, err)
	}

	if _, found, err := FindManualBackgroundRemovalDesign(designs, ""); err == nil || found {
		t.Fatalf("empty design id = (found %t, err %v), want validation error", found, err)
	}
	if _, found, err := FindManualBackgroundRemovalDesign(designs, "missing"); err != nil || found {
		t.Fatalf("missing design = (found %t, err %v), want (false, nil)", found, err)
	}
}

func TestFindManualBackgroundRemovalDesignRejectsPending(t *testing.T) {
	_, found, err := FindManualBackgroundRemovalDesign([]BackgroundRemovalDesign{{
		ID: "design-1", BackgroundRemovalStatus: BackgroundRemovalStatusPending,
	}}, "design-1")
	if err == nil || found {
		t.Fatalf("pending design = (found %t, err %v), want validation error", found, err)
	}
}

func TestPrepareManualBackgroundRemoval(t *testing.T) {
	fields, err := PrepareManualBackgroundRemoval(ManualBackgroundRemovalInput{
		DesignID:            "design-1",
		OriginalImageURL:    "  source.png ",
		ReplacementImageURL: "  removed.png ",
	})
	if err != nil {
		t.Fatalf("PrepareManualBackgroundRemoval() error = %v", err)
	}
	if fields.OriginalImageURL != "source.png" || fields.ImageURL != "removed.png" || fields.TransparentBackgroundMode != TransparencyModeRemoval || fields.BackgroundRemovalStatus != BackgroundRemovalStatusSucceeded || fields.BackgroundRemovalError != "" || fields.BackgroundRemovalModel != "" {
		t.Fatalf("fields = %#v, want normalized succeeded update", fields)
	}

	for name, input := range map[string]ManualBackgroundRemovalInput{
		"missing replacement": {DesignID: "design-1", OriginalImageURL: "source.png"},
		"missing source":      {DesignID: "design-1", ReplacementImageURL: "removed.png"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PrepareManualBackgroundRemoval(input); err == nil {
				t.Fatal("PrepareManualBackgroundRemoval() error = nil, want validation error")
			}
		})
	}
}
