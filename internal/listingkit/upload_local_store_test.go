package listingkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalImageUploadStoreSaveAndOpen(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	store, err := NewLocalImageUploadStore(rootDir)
	if err != nil {
		t.Fatalf("NewLocalImageUploadStore() error = %v", err)
	}

	input := &ImageUploadInput{
		Filename:    "shirt.jpg",
		ContentType: "image/jpeg",
		Data:        []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
	}

	file, err := store.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if file == nil {
		t.Fatal("Save() returned nil file")
	}
	if file.Key == "" {
		t.Fatal("Save() returned empty key")
	}
	if file.Path == "" {
		t.Fatal("Save() returned empty path")
	}
	if file.ContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", file.ContentType)
	}
	if file.Size != int64(len(input.Data)) {
		t.Fatalf("size = %d, want %d", file.Size, len(input.Data))
	}
	if _, err := os.Stat(file.Path); err != nil {
		t.Fatalf("uploaded file missing on disk: %v", err)
	}
	if filepath.Dir(file.Path) == rootDir {
		t.Fatalf("file path = %q, want nested storage path", file.Path)
	}

	opened, err := store.Open(context.Background(), file.Key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if opened.Path != file.Path {
		t.Fatalf("opened path = %q, want %q", opened.Path, file.Path)
	}
	if opened.ContentType != "image/jpeg" {
		t.Fatalf("opened content type = %q, want image/jpeg", opened.ContentType)
	}
}

func TestLocalImageUploadStoreOpenReturnsNotFound(t *testing.T) {
	t.Parallel()

	store, err := NewLocalImageUploadStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalImageUploadStore() error = %v", err)
	}

	_, err = store.Open(context.Background(), "missing/file.jpg")
	if err == nil {
		t.Fatal("Open() error = nil, want ErrUploadedImageNotFound")
	}
	if err != ErrUploadedImageNotFound {
		t.Fatalf("Open() error = %v, want %v", err, ErrUploadedImageNotFound)
	}
}
