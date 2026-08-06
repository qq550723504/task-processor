package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBuildS3ImageUploadStorePassesPublicBaseToS3Uploader(t *testing.T) {
	source, err := os.ReadFile("builders_image_store.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "PublicBase:") || !strings.Contains(string(source), "cfg.ProductImage.Publisher.PublicBase") {
		t.Fatal("builders_image_store.go must pass the configured public base to the S3 uploader")
	}
}
