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

func TestPlatformAssetPublisherRoutesAmazonOnlyToAmazon(t *testing.T) {
	t.Parallel()

	local := &recordingAssetPublisher{}
	amazon := &recordingAssetPublisher{}
	publisher := NewPlatformAssetPublisher(local, amazon)
	result := &ImageProcessResult{}

	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{TargetPlatform: "amazon"}, result))
	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{Marketplace: "amazon"}, result))
	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{TargetPlatform: "shein"}, result))
	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{TargetPlatform: "temu"}, result))

	require.Equal(t, 2, local.calls)
	require.Equal(t, 2, amazon.calls)
	require.Equal(t, []string{"shein", "temu"}, local.platforms)
	require.Equal(t, []string{"amazon", ""}, amazon.platforms)
}

func TestPlatformAssetPublisherSkipsNonAmazonWhenOnlyAmazonConfigured(t *testing.T) {
	t.Parallel()

	amazon := &recordingAssetPublisher{}
	publisher := NewPlatformAssetPublisher(nil, amazon)

	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{TargetPlatform: "shein"}, &ImageProcessResult{}))
	require.NoError(t, publisher.Publish(context.Background(), &ImageProcessRequest{TargetPlatform: "amazon"}, &ImageProcessResult{}))
	require.Equal(t, 1, amazon.calls)
	require.Equal(t, []string{"amazon"}, amazon.platforms)
}

func TestNewAmazonAssetPublisherBuildsFromExplicitOptions(t *testing.T) {
	t.Parallel()

	publisher, err := NewAmazonAssetPublisher(AmazonAssetPublisherOptions{
		Enabled:        true,
		Region:         "us-east-1",
		MarketplaceID:  "ATVPDKIKX0DER",
		ClientID:       "client-id",
		ClientSecret:   "client-secret",
		RefreshToken:   "refresh-token",
		AWSAccessKeyID: "access-key",
		AWSSecretKey:   "secret-key",
	})
	require.NoError(t, err)

	amazonPublisher, ok := publisher.(*amazonAssetPublisher)
	require.True(t, ok)
	require.Equal(t, "ATVPDKIKX0DER", amazonPublisher.marketplaceID)
}

func TestNewAmazonAssetPublisherRejectsDisabledOptions(t *testing.T) {
	t.Parallel()

	publisher, err := NewAmazonAssetPublisher(AmazonAssetPublisherOptions{})
	require.Nil(t, publisher)
	require.EqualError(t, err, "amazon SP-API is not enabled")
}

func TestS3AssetPublisherPublishSetsPublishedMetadata(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "main.jpg")
	require.NoError(t, os.WriteFile(sourcePath, []byte("main-image"), 0o644))

	publisher, err := NewS3AssetPublisher(S3AssetPublisherConfig{
		Uploader: &stubS3AssetUploader{
			url: "https://listingkit-assets.s3.amazonaws.com/productimage/task/main.jpg",
		},
		PublicBase: "https://cdn.example.com/productimage",
	})
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
	require.Equal(t, "s3", result.MainImage.Metadata["published_provider"])
	require.NotEmpty(t, result.MainImage.Metadata["published_key"])
	require.Contains(t, result.MainImage.URL, "https://cdn.example.com/productimage/")
}

type stubS3AssetUploader struct {
	url string
}

func (s *stubS3AssetUploader) Upload(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return s.url, nil
}

type recordingAssetPublisher struct {
	calls     int
	platforms []string
}

func (p *recordingAssetPublisher) Publish(_ context.Context, req *ImageProcessRequest, _ *ImageProcessResult) error {
	p.calls++
	if req != nil {
		p.platforms = append(p.platforms, req.TargetPlatform)
	}
	return nil
}
