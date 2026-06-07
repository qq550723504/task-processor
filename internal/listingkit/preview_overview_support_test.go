package listingkit

import "testing"

func TestPreviewPlatformsPrefersResultPlatforms(t *testing.T) {
	t.Parallel()

	got := previewPlatforms(&Task{
		Request: &GenerateRequest{Platforms: []string{"amazon"}},
		Result:  &ListingKitResult{Platforms: []string{"shein", "temu"}},
	})
	if len(got) != 2 || got[0] != "shein" || got[1] != "temu" {
		t.Fatalf("previewPlatforms() = %#v, want result platforms", got)
	}
}
