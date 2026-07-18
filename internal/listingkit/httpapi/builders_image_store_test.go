package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBuildS3ImageUploadStoreDoesNotConstructPublicObjectURL(t *testing.T) {
	source, err := os.ReadFile("builders_image_store.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"BuildS3PublicBase", "PublicBase:"} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("builders_image_store.go must not contain %q", forbidden)
		}
	}
}
