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
		"func findAssetURL(",
	} {
		if !strings.Contains(imageContent, signature) {
			t.Fatalf("platform_images.go should own %s", signature)
		}
	}

	helperSource, err := os.ReadFile("platform_helpers.go")
	if err != nil {
		t.Fatalf("read platform_helpers.go: %v", err)
	}
	helperContent := string(helperSource)
	for _, signature := range []string{
		"func buildPlatformImages(",
		"func buildPlatformImagesFromAssetBundle(",
		"func findAssetURL(",
	} {
		if strings.Contains(helperContent, signature) {
			t.Fatalf("platform_helpers.go should not own %s", signature)
		}
	}
}
