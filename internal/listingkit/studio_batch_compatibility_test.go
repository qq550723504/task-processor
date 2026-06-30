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

func TestStudioBatchCompatibilityFingerprint_DiffersWhenProductSizeDiffers(t *testing.T) {
	base := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: "https://cdn.example.com/template-a.png",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
		ProductSize:      `[[{"content":"尺码"},{"content":"衣长(cm/in)"}],[{"content":"S"},{"content":"87.5cm/34.45in"}]]`,
	}

	changed := base
	changed.ProductSize = `[[{"content":"尺码"},{"content":"衣长(cm/in)"}],[{"content":"S"},{"content":"90cm/35.43in"}]]`

	if got, want := buildStudioBatchCompatibilityFingerprint(base), buildStudioBatchCompatibilityFingerprint(changed); got == want {
		t.Fatalf("product-size-change fingerprints unexpectedly matched: %q", got)
	}
}

func TestStudioBatchTaskCandidateKey_DiffersWhenProductSizeDiffers(t *testing.T) {
	batch := &StudioBatchRecord{ID: "batch-1", TenantID: "tenant-1"}
	base := studioBatchTaskCandidate{
		Item:         StudioBatchItemRecord{ID: "item-1"},
		Design:       StudioMaterializedDesignRecord{ID: "design-1"},
		SelectionID:  "selection-1",
		SheinStoreID: 1043,
		SelectionSnapshot: SheinStudioSelection{
			ProductSize: `[[{"content":"尺码"},{"content":"衣长(cm/in)"}],[{"content":"S"},{"content":"87.5cm/34.45in"}]]`,
		},
	}
	changed := base
	changed.SelectionSnapshot.ProductSize = `[[{"content":"尺码"},{"content":"衣长(cm/in)"}],[{"content":"S"},{"content":"90cm/35.43in"}]]`

	if got, want := buildStudioBatchTaskCandidateKey(nil, batch, base), buildStudioBatchTaskCandidateKey(nil, batch, changed); got == want {
		t.Fatalf("product-size-change candidate keys unexpectedly matched: %q", got)
	}
}
