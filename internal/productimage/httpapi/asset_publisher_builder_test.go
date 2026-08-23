package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestNewAssetPublisherOptionsResolvesAmazonConfiguration(t *testing.T) {
	t.Parallel()

	options := newAssetPublisherOptions(&config.Config{Amazon: config.AmazonConfig{SPAPI: config.SPAPIConfig{
		Enabled:            true,
		Region:             "us-east-1",
		DefaultMarketplace: "us",
		Marketplaces: map[string]config.MarketplaceConfig{
			"us": {MarketplaceID: "ATVPDKIKX0DER", Enabled: true},
		},
		ClientID:       "client-id",
		ClientSecret:   "client-secret",
		RefreshToken:   "refresh-token",
		AWSAccessKeyID: "access-key",
		AWSSecretKey:   "secret-key",
	}}})

	require.True(t, options.amazon.Enabled)
	require.Equal(t, "ATVPDKIKX0DER", options.amazon.MarketplaceID)
	require.Equal(t, "us-east-1", options.amazon.Region)
}

func TestBuildAssetPublisherSupportsFilesystemAliases(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"file", "filesystem"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()

			publisher := buildAssetPublisher(assetPublisherOptions{
				enabled:    true,
				provider:   provider,
				outputDir:  t.TempDir(),
				publicBase: "https://cdn.example.com/assets",
			}, logrus.New())

			require.NotNil(t, publisher, "%s should use the local publisher implementation", provider)
		})
	}
}
