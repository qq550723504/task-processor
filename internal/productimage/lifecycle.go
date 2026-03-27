package productimage

import (
	"os"
	"path/filepath"
)

func canReuseAsset(asset *ImageAsset) bool {
	if asset == nil {
		return false
	}
	if asset.Metadata != nil {
		if publishedPath := asset.Metadata["published_path"]; publishedPath != "" {
			if _, err := os.Stat(publishedPath); err == nil {
				return true
			}
		}
		if uploadedURL := asset.Metadata["uploaded_url"]; uploadedURL != "" {
			return true
		}
		if localPath := asset.Metadata["local_path"]; localPath != "" {
			if _, err := os.Stat(localPath); err == nil {
				return true
			}
		}
	}
	if asset.URL != "" && !looksRemoteURL(asset.URL) {
		if _, err := os.Stat(asset.URL); err == nil {
			return true
		}
	}
	return false
}

func canReusePublishedAsset(asset *ImageAsset) bool {
	if asset == nil || asset.Metadata == nil {
		return false
	}
	if publishedPath := asset.Metadata["published_path"]; publishedPath != "" {
		if _, err := os.Stat(publishedPath); err == nil {
			return true
		}
	}
	if asset.Metadata["uploaded_url"] != "" || asset.Metadata["published_url"] != "" {
		return true
	}
	return false
}

func looksRemoteURL(value string) bool {
	return len(value) > 8 && (value[:7] == "http://" || value[:8] == "https://")
}

func cleanupTemporaryAssets(result *ImageProcessResult) {
	for _, asset := range collectAssets(result) {
		cleanupTemporaryAsset(asset)
	}
}

func cleanupTemporaryAsset(asset *ImageAsset) {
	if asset == nil || asset.Metadata == nil {
		return
	}
	localPath := asset.Metadata["local_path"]
	if localPath == "" {
		return
	}
	publishedPath := asset.Metadata["published_path"]
	if publishedPath != "" && sameAssetPath(localPath, publishedPath) {
		asset.Metadata["temp_file_cleaned"] = "skipped_same_as_published"
		return
	}
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		asset.Metadata["temp_file_cleaned"] = "false"
		asset.Metadata["temp_file_cleanup_error"] = err.Error()
		return
	}
	asset.Metadata["temp_file_cleaned"] = "true"
	asset.Metadata["temp_local_path"] = localPath
	delete(asset.Metadata, "local_path")
}

func sameAssetPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
