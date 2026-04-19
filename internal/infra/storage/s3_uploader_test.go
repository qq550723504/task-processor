package storage

import "testing"

func TestS3UploaderResolvedURLPrefersPublicBase(t *testing.T) {
	t.Parallel()

	uploader := NewS3UploaderWithOptions(nil, S3UploaderOptions{
		Bucket:     "listingkit-assets",
		PublicBase: "http://127.0.0.1:9100/listingkit-assets",
	})

	got := uploader.resolveObjectURL("20260419/example.jpg")
	want := "http://127.0.0.1:9100/listingkit-assets/20260419/example.jpg"
	if got != want {
		t.Fatalf("resolveObjectURL() = %q, want %q", got, want)
	}
}

func TestS3UploaderResolvedURLSupportsPathStyleEndpoint(t *testing.T) {
	t.Parallel()

	uploader := NewS3UploaderWithOptions(nil, S3UploaderOptions{
		Bucket:       "listingkit-assets",
		Endpoint:     "http://127.0.0.1:9100",
		UsePathStyle: true,
	})

	got := uploader.resolveObjectURL("20260419/example.jpg")
	want := "http://127.0.0.1:9100/listingkit-assets/20260419/example.jpg"
	if got != want {
		t.Fatalf("resolveObjectURL() = %q, want %q", got, want)
	}
}
