package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformImageProjectionHasDedicatedFile(t *testing.T) {
	for _, path := range []string{"temu_mapper.go", "walmart_mapper.go"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(source)
		if !strings.Contains(content, "common.BuildImagesWithSelection(") {
			t.Fatalf("%s should use common.BuildImagesWithSelection", path)
		}
		if strings.Contains(content, "buildPlatformImages(") {
			t.Fatalf("%s should not use the ListingKit image wrapper", path)
		}
	}

	if _, err := os.Stat("platform_images.go"); !os.IsNotExist(err) {
		t.Fatalf("platform_images.go should be removed, stat error = %v", err)
	}
}
