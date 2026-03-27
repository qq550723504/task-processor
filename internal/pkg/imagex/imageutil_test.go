package imagex

import (
	"encoding/base64"
	"testing"
)

func TestFromBytesWithFormat_DecodesWebP(t *testing.T) {
	const tinyWebPBase64 = "UklGRjwAAABXRUJQVlA4IDAAAADQAQCdASoCAAEAAgA0JaACdLoB+AADsAD+8Oj3/yC5YXXI1/8gP+QH/ID/+PIAAAA="

	data, err := base64.StdEncoding.DecodeString(tinyWebPBase64)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}

	img, format, err := FromBytesWithFormat(data)
	if err != nil {
		t.Fatalf("FromBytesWithFormat: %v", err)
	}
	if img == nil {
		t.Fatal("expected decoded image")
	}
	if format != "webp" {
		t.Fatalf("format = %q, want webp", format)
	}

	width, height := Size(img)
	if width <= 0 || height <= 0 {
		t.Fatalf("decoded size = %dx%d, want positive dimensions", width, height)
	}
}
