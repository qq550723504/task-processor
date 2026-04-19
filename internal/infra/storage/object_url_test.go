package storage

import "testing"

func TestBuildS3PublicBaseSupportsPathStyleEndpoint(t *testing.T) {
	t.Parallel()

	got := BuildS3PublicBase("https://s3.example.com", "listingkit-assets", true)
	want := "https://s3.example.com/listingkit-assets"
	if got != want {
		t.Fatalf("BuildS3PublicBase() = %q, want %q", got, want)
	}
}

func TestBuildS3PublicBaseSupportsVirtualHostedEndpoint(t *testing.T) {
	t.Parallel()

	got := BuildS3PublicBase("https://s3.example.com", "listingkit-assets", false)
	want := "https://listingkit-assets.s3.example.com"
	if got != want {
		t.Fatalf("BuildS3PublicBase() = %q, want %q", got, want)
	}
}
