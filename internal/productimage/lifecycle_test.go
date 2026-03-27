package productimage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanupTemporaryAssets_RemovesTempFileWhenPublishedExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	localPath := filepath.Join(dir, "temp-main.jpg")
	publishedPath := filepath.Join(dir, "published", "main.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(publishedPath), 0o755))
	require.NoError(t, os.WriteFile(localPath, []byte("temp"), 0o644))
	require.NoError(t, os.WriteFile(publishedPath, []byte("published"), 0o644))

	result := &ImageProcessResult{
		MainImage: &ImageAsset{
			Type: AssetTypeMainImage,
			Metadata: map[string]string{
				"local_path":     localPath,
				"published_path": publishedPath,
			},
		},
	}

	cleanupTemporaryAssets(result)

	_, err := os.Stat(localPath)
	require.True(t, os.IsNotExist(err))
	require.Equal(t, "true", result.MainImage.Metadata["temp_file_cleaned"])
	require.Equal(t, localPath, result.MainImage.Metadata["temp_local_path"])
	_, ok := result.MainImage.Metadata["local_path"]
	require.False(t, ok)
}

func TestCleanupTemporaryAssets_KeepsFileWhenPublishedPathMatchesLocal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	localPath := filepath.Join(dir, "main.jpg")
	require.NoError(t, os.WriteFile(localPath, []byte("asset"), 0o644))

	result := &ImageProcessResult{
		MainImage: &ImageAsset{
			Type: AssetTypeMainImage,
			Metadata: map[string]string{
				"local_path":     localPath,
				"published_path": localPath,
			},
		},
	}

	cleanupTemporaryAssets(result)

	_, err := os.Stat(localPath)
	require.NoError(t, err)
	require.Equal(t, "skipped_same_as_published", result.MainImage.Metadata["temp_file_cleaned"])
	require.Equal(t, localPath, result.MainImage.Metadata["local_path"])
}

func TestCanReuseAssetAndPublishedAsset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	localPath := filepath.Join(dir, "main.jpg")
	require.NoError(t, os.WriteFile(localPath, []byte("asset"), 0o644))

	asset := &ImageAsset{
		Type: AssetTypeMainImage,
		URL:  localPath,
		Metadata: map[string]string{
			"local_path": localPath,
		},
	}
	require.True(t, canReuseAsset(asset))
	require.False(t, canReusePublishedAsset(asset))

	asset.Metadata["uploaded_url"] = "https://cdn.example.com/main.jpg"
	require.True(t, canReusePublishedAsset(asset))
}
