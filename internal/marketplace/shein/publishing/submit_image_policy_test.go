package publishing

import "testing"

func TestFinalSubmitImagesRequireSKCSkipsSaveDraftOnly(t *testing.T) {
	t.Parallel()

	if FinalSubmitImagesRequireSKC(" save_draft ") {
		t.Fatal("FinalSubmitImagesRequireSKC(save_draft) = true, want false")
	}
	for _, action := range []string{"publish", "", "unknown"} {
		if !FinalSubmitImagesRequireSKC(action) {
			t.Fatalf("FinalSubmitImagesRequireSKC(%q) = false, want true", action)
		}
	}
}

func TestFinalSubmitImagesReadyHandlesLegacyAndRequiredImages(t *testing.T) {
	t.Parallel()

	if ready, _ := FinalSubmitImagesReady("publish", FinalSubmitImageReadinessInput{}); !ready {
		t.Fatal("legacy final draft readiness = false, want true")
	}
	if ready, message := FinalSubmitImagesReady("publish", FinalSubmitImageReadinessInput{HasFinalDraft: true}); ready || message == "" {
		t.Fatalf("missing main image = (%v, %q), want blocker", ready, message)
	}
	if ready, message := FinalSubmitImagesReady("publish", FinalSubmitImageReadinessInput{
		HasFinalDraft: true,
		HasMainImage:  true,
	}); ready || message == "" {
		t.Fatalf("missing gallery = (%v, %q), want blocker", ready, message)
	}
}

func TestFinalSubmitImagesReadyUsesActionSpecificSKCStrictness(t *testing.T) {
	t.Parallel()

	base := FinalSubmitImageReadinessInput{
		HasFinalDraft: true,
		HasMainImage:  true,
		HasGallery:    true,
	}
	if ready, message := FinalSubmitImagesReady("save_draft", base); !ready || message == "" {
		t.Fatalf("save draft readiness = (%v, %q), want ready", ready, message)
	}
	base.RequiresSKC = true
	if ready, message := FinalSubmitImagesReady("save_draft", base); ready || message == "" {
		t.Fatalf("save draft with explicit SKC requirement = (%v, %q), want blocker", ready, message)
	}
	base.RequiresSKC = false
	if ready, message := FinalSubmitImagesReady("publish", base); ready || message == "" {
		t.Fatalf("publish without SKC = (%v, %q), want blocker", ready, message)
	}
	base.HasSKCImage = true
	if ready, message := FinalSubmitImagesReady("publish", base); ready || message == "" {
		t.Fatalf("publish without swatch = (%v, %q), want blocker", ready, message)
	}
	base.HasSwatchRole = true
	if ready, message := FinalSubmitImagesReady("publish", base); !ready || message == "" {
		t.Fatalf("publish ready = (%v, %q), want ready", ready, message)
	}
}

func TestImageURLClassifiersRecognizeUploadedAndSDSHosts(t *testing.T) {
	t.Parallel()

	if !IsUploadedImageURL(" https://img.shein.com/uploaded.jpg ") {
		t.Fatal("IsUploadedImageURL(shein image) = false, want true")
	}
	if !IsUploadedImageURL("https://cdn.ltwebstatic.com/uploaded.jpg") {
		t.Fatal("IsUploadedImageURL(ltwebstatic image) = false, want true")
	}
	if IsUploadedImageURL("https://cdn.example.com/source.jpg") {
		t.Fatal("IsUploadedImageURL(source image) = true, want false")
	}
	if !IsSDSImageURL("https://cdn.sdspod.com/source.jpg") {
		t.Fatal("IsSDSImageURL(sdspod image) = false, want true")
	}
	if !IsSDSImageURL("https://asset.sdsdiy.com/source.jpg") {
		t.Fatal("IsSDSImageURL(sdsdiy image) = false, want true")
	}
	if IsSDSImageURL("https://img.shein.com/uploaded.jpg") {
		t.Fatal("IsSDSImageURL(uploaded image) = true, want false")
	}
}

func TestCloneImageUploadCacheKeepsOnlyUploadedEntries(t *testing.T) {
	t.Parallel()

	cloned := CloneImageUploadCache(map[string]string{
		" https://cdn.example.com/source.jpg ": " https://img.shein.com/uploaded.jpg ",
		"https://cdn.example.com/bad.jpg":      "https://cdn.example.com/not-uploaded.jpg",
		"":                                     "https://img.shein.com/empty-key.jpg",
	})
	if len(cloned) != 1 || cloned["https://cdn.example.com/source.jpg"] != "https://img.shein.com/uploaded.jpg" {
		t.Fatalf("CloneImageUploadCache() = %#v, want only normalized uploaded cache entry", cloned)
	}

	empty := CloneImageUploadCache(nil)
	if len(empty) != 0 {
		t.Fatalf("CloneImageUploadCache(nil) = %#v, want empty map", empty)
	}
	empty["source"] = "uploaded"
}
