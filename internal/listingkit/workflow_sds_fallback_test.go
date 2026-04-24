package listingkit

import "testing"

func TestStudioSDSMaterialFileNameUsesTaskID(t *testing.T) {
	t.Parallel()

	got := studioSDSMaterialFileName(&Task{ID: "a0991cd2-f5d0-439a-bde7-f0530591ab12"})
	if got != "listingkit-studio-design-a0991cd2.png" {
		t.Fatalf("filename = %q", got)
	}
}

func TestNeedsLocalSDSMockupFallbackWhenSDSReturnsTooFewImages(t *testing.T) {
	t.Parallel()

	summary := &SDSSyncSummary{
		MockupImageURLs: []string{"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg"},
	}
	options := &SDSSyncOptions{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/images/mockup-1.jpg",
			"https://cdn.sdspod.com/images/mockup-2.jpg",
			"https://cdn.sdspod.com/images/mockup-3.jpg",
		},
	}

	if !needsLocalSDSMockupFallback(summary, options) {
		t.Fatal("expected local fallback when SDS returns fewer images than selected mockups")
	}
}

func TestNeedsLocalSDSMockupFallbackSkipsCompleteSDSSet(t *testing.T) {
	t.Parallel()

	summary := &SDSSyncSummary{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
			"https://cdn.sdspod.com/out/0/202604/rendered-gallery.jpg",
		},
	}
	options := &SDSSyncOptions{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/images/mockup-1.jpg",
			"https://cdn.sdspod.com/images/mockup-2.jpg",
		},
	}

	if needsLocalSDSMockupFallback(summary, options) {
		t.Fatal("did not expect local fallback when SDS returns a complete image set")
	}
}
