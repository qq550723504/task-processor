package httpapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
)

func TestBuildImageUploadStoreUsesOnlyListingKitLocalConfiguration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "listingkit-inputs")
	cfg := &config.Config{
		ListingKit: config.ListingKitConfig{ImageUpload: config.ListingKitImageUploadConfig{
			Provider: "local",
			Local:    config.ListingKitImageUploadLocalConfig{RootDir: root},
		}},
		ImageAgent: config.ImageAgentConfig{ArtifactStore: config.ImageAgentArtifactStoreConfig{
			Provider:   "s3",
			PublicBase: "https://must-not-be-read.example.test",
		}},
	}

	store, err := BuildImageUploadStore(cfg, logrus.New())
	require.NoError(t, err)
	require.NotNil(t, store)
	first, err := store.Save(t.Context(), &listingkit.ImageUploadInput{Filename: "first.jpg", Data: []byte{0xff, 0xd8, 0xff}})
	require.NoError(t, err)

	cfg.ImageAgent.ArtifactStore.Provider = "local"
	cfg.ImageAgent.ArtifactStore.PublicBase = "https://mutated.example.test"
	store, err = BuildImageUploadStore(cfg, logrus.New())
	require.NoError(t, err)
	second, err := store.Save(t.Context(), &listingkit.ImageUploadInput{Filename: "second.jpg", Data: []byte{0xff, 0xd8, 0xff}})
	require.NoError(t, err)

	require.NotEqual(t, first.Path, second.Path)
	require.FileExists(t, first.Path)
	require.FileExists(t, second.Path)
	require.Equal(t, root, filepath.Dir(filepath.Dir(first.Path)))
	require.Equal(t, root, filepath.Dir(filepath.Dir(second.Path)))
}

func TestBuildImageUploadStoreFailsClosedWhenSelectedS3IsInvalid(t *testing.T) {
	localRoot := filepath.Join(t.TempDir(), "must-not-exist")
	cfg := &config.Config{ListingKit: config.ListingKitConfig{ImageUpload: config.ListingKitImageUploadConfig{
		Provider: "s3",
		Local:    config.ListingKitImageUploadLocalConfig{RootDir: localRoot},
		S3:       config.ListingKitImageUploadS3Config{Region: "ap-southeast-1"},
	}}}

	store, err := BuildImageUploadStore(cfg, logrus.New())

	require.ErrorContains(t, err, "listingkit.imageUpload.s3.bucket")
	require.Nil(t, store)
	_, statErr := os.Stat(localRoot)
	require.ErrorIs(t, statErr, os.ErrNotExist, "S3 construction must not create a local secondary store")
}

func TestBuildSubmitModulePropagatesSelectedS3ConstructionFailure(t *testing.T) {
	localRoot := filepath.Join(t.TempDir(), "must-not-exist")
	cfg := &config.Config{ListingKit: config.ListingKitConfig{ImageUpload: config.ListingKitImageUploadConfig{
		Provider: "s3",
		Local:    config.ListingKitImageUploadLocalConfig{RootDir: localRoot},
		S3:       config.ListingKitImageUploadS3Config{Region: "ap-southeast-1"},
	}}}

	module, err := buildSubmitModule(submitModuleInput{
		Config: cfg,
		Logger: logrus.New(),
		Hooks:  submitModuleHooks{ImageUploadStoreBuilder: BuildImageUploadStore},
	})

	require.ErrorContains(t, err, "build ListingKit image upload store")
	require.ErrorContains(t, err, "listingkit.imageUpload.s3.bucket")
	require.Equal(t, submitModule{}, module)
	_, statErr := os.Stat(localRoot)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
