package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAmazonMarketplaceID_PrefersConfiguredMarketKey(t *testing.T) {
	spapi := SPAPIConfig{
		DefaultMarketplace: "us",
		Marketplaces: map[string]MarketplaceConfig{
			"us": {MarketplaceID: "ATVPDKIKX0DER", Enabled: true},
		},
	}

	assert.Equal(t, "ATVPDKIKX0DER", ResolveAmazonMarketplaceID(spapi))
}

func TestResolveAmazonMarketplaceID_AcceptsMarketplaceID(t *testing.T) {
	spapi := SPAPIConfig{
		DefaultMarketplace: "ATVPDKIKX0DER",
		Marketplaces: map[string]MarketplaceConfig{
			"us": {MarketplaceID: "ATVPDKIKX0DER", Enabled: true},
		},
	}

	assert.Equal(t, "ATVPDKIKX0DER", ResolveAmazonMarketplaceID(spapi))
}

func TestResolveAmazonMarketplaceConfig_ResolvesByIDOrKey(t *testing.T) {
	spapi := SPAPIConfig{
		DefaultMarketplace: "ATVPDKIKX0DER",
		Marketplaces: map[string]MarketplaceConfig{
			"us": {MarketplaceID: "ATVPDKIKX0DER", SellerID: "SELLER-US", Enabled: true},
		},
	}

	cfgByID := ResolveAmazonMarketplaceConfig(spapi, "ATVPDKIKX0DER")
	if assert.NotNil(t, cfgByID) {
		assert.Equal(t, "SELLER-US", cfgByID.SellerID)
	}

	cfgByKey := ResolveAmazonMarketplaceConfig(spapi, "us")
	if assert.NotNil(t, cfgByKey) {
		assert.Equal(t, "SELLER-US", cfgByKey.SellerID)
	}
}
