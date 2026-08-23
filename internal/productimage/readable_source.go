package productimage

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ReadableAssetSource is the source that image processing should actually
// read. It deliberately separates current readable output from source
// provenance, because a published/uploaded asset may no longer be identical
// to the original marketplace image.
type ReadableAssetSource struct {
	LocalPath string
	URL       string
}

// ResolveReadableAssetURL applies the URL portion of the readable-source
// contract to callers that still hold an asset record instead of an
// ImageAsset. The current processed/published URL always outranks provenance.
func ResolveReadableAssetURL(currentURL, provenanceURL string, metadata map[string]string) string {
	for _, candidate := range []string{
		metadata["readable_url"],
		metadata["published_url"],
		metadata["uploaded_url"],
		currentURL,
		provenanceURL,
	} {
		if isLegacyAmazonUploadURL(candidate, metadata) {
			continue
		}
		if isHTTPURL(candidate) {
			return strings.TrimSpace(candidate)
		}
	}
	if strings.TrimSpace(currentURL) != "" {
		return strings.TrimSpace(currentURL)
	}
	return strings.TrimSpace(provenanceURL)
}

func isLegacyAmazonUploadURL(candidate string, metadata map[string]string) bool {
	if !strings.EqualFold(strings.TrimSpace(metadata["published_provider"]), "amazon") {
		return false
	}
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	for _, key := range []string{"uploaded_url", "uploaded_destination_url"} {
		if candidate == strings.TrimSpace(metadata[key]) {
			return true
		}
	}
	return false
}

// ResolveReadableAssetSource returns the best currently readable source for
// an asset. Durable local and published outputs outrank remote URLs; the
// original SourceURL is only a last-resort fallback.
func ResolveReadableAssetSource(asset *ImageAsset) (ReadableAssetSource, error) {
	if asset == nil {
		return ReadableAssetSource{}, fmt.Errorf("asset cannot be nil")
	}

	for _, key := range []string{"local_path", "published_path"} {
		path := strings.TrimSpace(asset.Metadata[key])
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return ReadableAssetSource{LocalPath: path}, nil
		}
	}

	if readableURL := ResolveReadableAssetURL(asset.URL, asset.SourceURL, asset.Metadata); isHTTPURL(readableURL) {
		return ReadableAssetSource{URL: readableURL}, nil
	}

	for _, candidate := range []string{asset.URL, asset.SourceURL} {
		path := strings.TrimSpace(candidate)
		if path == "" || isHTTPURL(path) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return ReadableAssetSource{LocalPath: path}, nil
		}
	}

	return ReadableAssetSource{}, fmt.Errorf("asset has no readable source")
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
