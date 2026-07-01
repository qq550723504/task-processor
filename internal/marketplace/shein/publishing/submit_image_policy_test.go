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

func TestSubmitImageDraftHasImageChecksAllImageSources(t *testing.T) {
	t.Parallel()

	if SubmitImageDraftHasImage(SubmitImageDraftInput{}) {
		t.Fatal("empty draft = true, want false")
	}
	if !SubmitImageDraftHasImage(SubmitImageDraftInput{MainImage: " https://cdn.example/main.jpg "}) {
		t.Fatal("main image draft = false, want true")
	}
	if !SubmitImageDraftHasImage(SubmitImageDraftInput{Gallery: []string{"", "https://cdn.example/gallery.jpg"}}) {
		t.Fatal("gallery image draft = false, want true")
	}
	if !SubmitImageDraftHasImage(SubmitImageDraftInput{Source: []string{"https://cdn.example/source.jpg"}}) {
		t.Fatal("source image draft = false, want true")
	}
}

func TestNormalizeImageRoleOverridesKeepsAcceptedRoles(t *testing.T) {
	t.Parallel()

	roles := NormalizeImageRoleOverrides(map[string]string{
		" https://cdn.example/main.jpg ": " MAIN ",
		"https://cdn.example/skc.jpg":    "skc",
		"https://cdn.example/nope.jpg":   "invalid",
		" ":                              "swatch",
	})

	if roles["https://cdn.example/main.jpg"] != "main" {
		t.Fatalf("main role = %q, want normalized main", roles["https://cdn.example/main.jpg"])
	}
	if roles["https://cdn.example/skc.jpg"] != "skc" {
		t.Fatalf("skc role = %q, want skc", roles["https://cdn.example/skc.jpg"])
	}
	if _, ok := roles["https://cdn.example/nope.jpg"]; ok {
		t.Fatalf("invalid role kept: %#v", roles)
	}
}

func TestUniqueNonEmptyImageURLsTrimsAndDedupes(t *testing.T) {
	t.Parallel()

	got := UniqueNonEmptyImageURLs([]string{" a.jpg ", "", "b.jpg", "a.jpg", " b.jpg "})
	want := []string{"a.jpg", "b.jpg"}
	if len(got) != len(want) {
		t.Fatalf("UniqueNonEmptyImageURLs() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("UniqueNonEmptyImageURLs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOrderFinalDraftImagesAppliesOrderDeletedAndDedupes(t *testing.T) {
	t.Parallel()

	got := OrderFinalDraftImages(
		[]string{" existing-a.jpg ", "ordered.jpg", "deleted.jpg", "existing-a.jpg", ""},
		[]string{" ordered.jpg ", "deleted.jpg", "ordered.jpg", "later.jpg"},
		map[string]struct{}{"deleted.jpg": {}},
	)
	want := []string{"ordered.jpg", "later.jpg", "existing-a.jpg"}
	if len(got) != len(want) {
		t.Fatalf("OrderFinalDraftImages() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OrderFinalDraftImages()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFirstNonEmptyImageURLPreservesReturnedValue(t *testing.T) {
	t.Parallel()

	got := FirstNonEmptyImageURL("", "   ", " https://cdn.example/image.jpg ", "later.jpg")
	if got != " https://cdn.example/image.jpg " {
		t.Fatalf("FirstNonEmptyImageURL() = %q, want original first non-empty value", got)
	}
}

func TestGalleryWithoutMainTrimsAndFiltersMain(t *testing.T) {
	t.Parallel()

	got := GalleryWithoutMain([]string{" main.jpg ", "gallery.jpg", "", "main.jpg"}, "main.jpg")
	want := []string{"gallery.jpg"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("GalleryWithoutMain() = %#v, want %#v", got, want)
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
