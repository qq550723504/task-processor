package preview

import (
	"os"
	"testing"
)

func TestPreviewPackageDoesNotOwnPlatformCompatibilityWrappers(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"platform.go",
		"sections.go",
	} {
		if _, err := os.Stat(file); err == nil {
			t.Fatalf("%s should be removed after listing platform framework extraction", file)
		} else if !os.IsNotExist(err) {
			t.Fatalf("Stat(%s) unexpected error = %v", file, err)
		}
	}
}
