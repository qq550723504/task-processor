package httpapi

import (
	"testing"

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
