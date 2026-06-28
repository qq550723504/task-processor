package preview

import (
	"os"
	"strings"
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

func TestPreviewPackageDoesNotAliasPlatformErrors(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("errors.go")
	if err != nil {
		t.Fatalf("ReadFile(errors.go) error = %v", err)
	}
	content := string(src)

	for _, needle := range []string{
		`"task-processor/internal/listing/platform"`,
		"ErrUnsupportedPlatform",
		"ErrPlatformUnavailable",
	} {
		if strings.Contains(content, needle) {
			t.Fatalf("errors.go should not contain %q after platform error extraction", needle)
		}
	}

	if !strings.Contains(content, `errors.New("task result unavailable")`) {
		t.Fatal("errors.go should keep the preview task-result availability sentinel")
	}
}
