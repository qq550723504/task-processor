package listingkit

import "testing"

func TestStudioBatchCompatibilityFingerprint_MatchesEquivalentSelections(t *testing.T) {
	left := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: " https://cdn.example.com/template-a.png ",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}
	right := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: "https://cdn.example.com/template-a.png",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}

	if got, want := buildStudioBatchCompatibilityFingerprint(left), buildStudioBatchCompatibilityFingerprint(right); got != want {
		t.Fatalf("fingerprint mismatch: %q != %q", got, want)
	}
}

func TestStudioBatchCompatibilityFingerprint_DiffersWhenTemplateOrMaskDiffers(t *testing.T) {
	base := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: "https://cdn.example.com/template-a.png",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}

	changedTemplate := base
	changedTemplate.TemplateImageURL = "https://cdn.example.com/template-b.png"
	if got, want := buildStudioBatchCompatibilityFingerprint(base), buildStudioBatchCompatibilityFingerprint(changedTemplate); got == want {
		t.Fatalf("template-change fingerprints unexpectedly matched: %q", got)
	}

	changedMask := base
	changedMask.MaskImageURL = "https://cdn.example.com/mask-b.png"
	if got, want := buildStudioBatchCompatibilityFingerprint(base), buildStudioBatchCompatibilityFingerprint(changedMask); got == want {
		t.Fatalf("mask-change fingerprints unexpectedly matched: %q", got)
	}
}
