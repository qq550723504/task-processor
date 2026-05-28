package listingkit

import (
	"context"
	"errors"
	"testing"
)

func TestFallbackImageUploadStoreOpenFallsBackToSecondaryStore(t *testing.T) {
	t.Parallel()

	store := &fallbackImageUploadStore{
		primary: &stubFallbackImageUploadStore{
			openErr: ErrUploadedImageNotFound,
		},
		fallback: &stubFallbackImageUploadStore{
			openResult: &StoredUploadedImage{
				Key:         "20260529/demo.png",
				Filename:    "demo.png",
				ContentType: "image/png",
				Data:        []byte("fallback-image"),
			},
		},
	}

	file, err := store.Open(context.Background(), "20260529/demo.png")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if file == nil {
		t.Fatal("Open() returned nil file")
	}
	if string(file.Data) != "fallback-image" {
		t.Fatalf("Open() data = %q, want fallback-image", string(file.Data))
	}
}

func TestFallbackImageUploadStoreSaveUsesPrimaryStore(t *testing.T) {
	t.Parallel()

	primary := &stubFallbackImageUploadStore{
		saveResult: &StoredUploadedImage{
			Key:       "20260529/demo.png",
			PublicURL: "https://oss.example.com/listingkit-assets/20260529/demo.png",
		},
	}
	fallback := &stubFallbackImageUploadStore{}
	store := &fallbackImageUploadStore{
		primary:  primary,
		fallback: fallback,
	}

	file, err := store.Save(context.Background(), &ImageUploadInput{
		Filename:    "demo.png",
		ContentType: "image/png",
		Data:        []byte("image"),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if file == nil || file.PublicURL == "" {
		t.Fatal("Save() should return the primary store result")
	}
	if primary.saveCalls != 1 {
		t.Fatalf("primary save calls = %d, want 1", primary.saveCalls)
	}
	if fallback.saveCalls != 0 {
		t.Fatalf("fallback save calls = %d, want 0", fallback.saveCalls)
	}
}

type stubFallbackImageUploadStore struct {
	openResult *StoredUploadedImage
	openErr    error
	saveResult *StoredUploadedImage
	saveErr    error
	saveCalls  int
}

func (s *stubFallbackImageUploadStore) Save(_ context.Context, _ *ImageUploadInput) (*StoredUploadedImage, error) {
	s.saveCalls += 1
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	return s.saveResult, nil
}

func (s *stubFallbackImageUploadStore) Open(_ context.Context, _ string) (*StoredUploadedImage, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	if s.openResult == nil {
		return nil, ErrUploadedImageNotFound
	}
	return s.openResult, nil
}

func (s *stubFallbackImageUploadStore) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}
