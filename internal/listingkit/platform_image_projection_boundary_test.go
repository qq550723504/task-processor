package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformImageProjectionHasDedicatedFile(t *testing.T) {
	imageSource, err := os.ReadFile("platform_images.go")
	if err != nil {
		t.Fatalf("read platform_images.go: %v", err)
	}
	imageContent := string(imageSource)
	for _, signature := range []string{
		"func buildPlatformImages(",
		"func buildPlatformImagesFromAssetBundle(",
	} {
		if !strings.Contains(imageContent, signature) {
			t.Fatalf("platform_images.go should own %s", signature)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	for _, signature := range []string{
		"func buildPlatformImages(",
		"func buildPlatformImagesFromAssetBundle(",
	} {
		if strings.Contains(string(helperSource), signature) {
			t.Fatalf("platform_helpers.go should not own %s", signature)
		}
	}
	if !strings.Contains(imageContent, "common.BuildImagesFromBundleWithSelection(") {
		t.Fatal("platform_images.go should delegate bundle projection to common selection-aware builder")
	}
	if strings.Contains(imageContent, "func findAssetURL(") {
		t.Fatal("platform_images.go should not duplicate asset URL lookup")
	}
}
