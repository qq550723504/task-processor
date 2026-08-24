package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestRenderPreviewCapabilitiesUseGenerationDomainDirectly(t *testing.T) {
	source, err := os.ReadFile("generation_render_preview_capabilities.go")
	if err != nil {
		t.Fatalf("read generation_render_preview_capabilities.go: %v", err)
	}
	if strings.Contains(string(source), "func buildRenderPreviewCapabilities(item GenerationWorkQueueItem)") {
		t.Fatal("generation_render_preview_capabilities.go should not keep the direct forwarding wrapper")
	}

	for _, path := range []string{"generation_queue_bundle_support.go", "generation_review_state.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if strings.Contains(content, "buildRenderPreviewCapabilities(") {
			t.Fatalf("%s should not use the ListingKit forwarding wrapper", path)
		}
		if !strings.Contains(content, "listinggeneration.RenderPreviewCapabilities(") {
			t.Fatalf("%s should call the generation domain directly", path)
		}
	}
}
