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
