package common

import "testing"

func TestCloneImageSetDeepCopiesSlices(t *testing.T) {
	t.Parallel()

	src := &ImageSet{
		MainImage:    "https://cdn.example.com/main.jpg",
		WhiteBgImage: "https://cdn.example.com/white.jpg",
		Gallery:      []string{"https://cdn.example.com/1.jpg"},
		SourceImages: []string{"https://cdn.example.com/source.jpg"},
	}

	cloned := CloneImageSet(src)
	if cloned == nil {
		t.Fatal("CloneImageSet() = nil, want clone")
	}
	src.Gallery[0] = "https://cdn.example.com/changed-gallery.jpg"
	src.SourceImages[0] = "https://cdn.example.com/changed-source.jpg"

	if cloned.MainImage != "https://cdn.example.com/main.jpg" || cloned.WhiteBgImage != "https://cdn.example.com/white.jpg" {
		t.Fatalf("clone scalar fields = %+v, want original values", cloned)
	}
	if cloned.Gallery[0] != "https://cdn.example.com/1.jpg" {
		t.Fatalf("clone gallery = %+v, want deep copy", cloned.Gallery)
	}
	if cloned.SourceImages[0] != "https://cdn.example.com/source.jpg" {
		t.Fatalf("clone source images = %+v, want deep copy", cloned.SourceImages)
	}
}
