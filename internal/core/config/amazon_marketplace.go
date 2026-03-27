package config

import "strings"

func ResolveAmazonMarketplaceID(spapi SPAPIConfig) string {
	key := strings.TrimSpace(spapi.DefaultMarketplace)
	if key == "" {
		return firstEnabledAmazonMarketplaceID(spapi.Marketplaces)
	}

	if market, ok := spapi.Marketplaces[key]; ok && strings.TrimSpace(market.MarketplaceID) != "" {
		return strings.TrimSpace(market.MarketplaceID)
	}

	for _, market := range spapi.Marketplaces {
		if strings.EqualFold(strings.TrimSpace(market.MarketplaceID), key) {
			return strings.TrimSpace(market.MarketplaceID)
		}
	}

	return key
}

func ResolveAmazonMarketplaceConfig(spapi SPAPIConfig, marketplace string) *MarketplaceConfig {
	normalized := strings.TrimSpace(marketplace)
	if normalized == "" {
		normalized = ResolveAmazonMarketplaceID(spapi)
	}

	if normalized == "" {
		return nil
	}

	if market, ok := spapi.Marketplaces[normalized]; ok {
		m := market
		return &m
	}

	for key, market := range spapi.Marketplaces {
		if strings.EqualFold(strings.TrimSpace(market.MarketplaceID), normalized) || strings.EqualFold(strings.TrimSpace(key), normalized) {
			m := market
			return &m
		}
	}

	return nil
}

func firstEnabledAmazonMarketplaceID(markets map[string]MarketplaceConfig) string {
	for _, market := range markets {
		if market.Enabled && strings.TrimSpace(market.MarketplaceID) != "" {
			return strings.TrimSpace(market.MarketplaceID)
		}
	}
	for _, market := range markets {
		if strings.TrimSpace(market.MarketplaceID) != "" {
			return strings.TrimSpace(market.MarketplaceID)
		}
	}
	return ""
}
