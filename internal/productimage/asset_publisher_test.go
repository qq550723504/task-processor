package productimage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalAssetPublisher_Publish(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "main.jpg")
	require.NoError(t, os.WriteFile(sourcePath, []byte("main-image"), 0o644))

	publisher, err := NewLocalAssetPublisher(filepath.Join(workDir, "published"), "https://cdn.example.com/productimage")
	require.NoError(t, err)

	result := &ImageProcessResult{
		MainImage: &ImageAsset{
			URL:      sourcePath,
			Type:     AssetTypeMainImage,
			Metadata: map[string]string{"local_path": sourcePath},
		},
	}

	err = publisher.Publish(context.Background(), &ImageProcessRequest{ProductURL: "https://detail.1688.com/offer/123.html"}, result)
	require.NoError(t, err)
	require.NotNil(t, result.MainImage)
	require.Equal(t, "local", result.MainImage.Metadata["published_provider"])
	require.FileExists(t, result.MainImage.Metadata["published_path"])
	require.Contains(t, result.MainImage.URL, "https://cdn.example.com/productimage/")
	require.NotEmpty(t, result.MainImage.Metadata["published_key"])
}

func TestNewMultiAssetPublisher_SkipsNil(t *testing.T) {
	t.Parallel()

	publisher := NewMultiAssetPublisher(nil)
	require.Nil(t, publisher)
}
