package amazonlisting

import (
	"context"
	"fmt"
	"strings"

	amazonapi "task-processor/internal/amazon/api"
	coreconfig "task-processor/internal/core/config"
)

type spAPISubmitter struct {
	cfg *coreconfig.Config
}

func NewSPAPISubmitter(cfg *coreconfig.Config) ListingSubmitter {
	if cfg == nil || !cfg.Amazon.SPAPI.Enabled {
		return nil
	}
	return &spAPISubmitter{cfg: cfg}
}

func (s *spAPISubmitter) Preview(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.ValidationPreviewRequest == nil {
		return nil, fmt.Errorf("validation preview request is empty")
	}
	return client.ValidateListing(ctx, export.ValidationPreviewRequest)
}

func (s *spAPISubmitter) Create(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.CreateRequest == nil {
		return nil, fmt.Errorf("create request is empty")
	}
	return client.CreateListing(ctx, export.CreateRequest)
}

func (s *spAPISubmitter) Update(ctx context.Context, export *AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.UpdateRequest == nil {
		return nil, fmt.Errorf("update request is empty")
	}
	return client.UpdateListing(ctx, export.UpdateRequest)
}

func (s *spAPISubmitter) newClient(export *AmazonListingsAPIExport) (*amazonapi.Client, error) {
	if s.cfg == nil {
		return nil, fmt.Errorf("config is not available")
	}
	if export == nil {
		return nil, fmt.Errorf("listing export is empty")
	}
	marketplaceID := strings.TrimSpace(export.MarketplaceID)
	if marketplaceID == "" {
		return nil, fmt.Errorf("marketplace id is empty")
	}

	marketCfg, err := resolveMarketplaceConfig(s.cfg, marketplaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(marketCfg.SellerID) == "" {
		return nil, fmt.Errorf("seller id is not configured for marketplace %s", marketplaceID)
	}

	client := amazonapi.NewClient(&amazonapi.Config{
		Region:         s.cfg.Amazon.SPAPI.Region,
		MarketplaceID:  marketplaceID,
		SellerID:       marketCfg.SellerID,
		ClientID:       s.cfg.Amazon.SPAPI.ClientID,
		ClientSecret:   s.cfg.Amazon.SPAPI.ClientSecret,
		RefreshToken:   s.cfg.Amazon.SPAPI.RefreshToken,
		AWSAccessKeyID: s.cfg.Amazon.SPAPI.AWSAccessKeyID,
		AWSSecretKey:   s.cfg.Amazon.SPAPI.AWSSecretKey,
	})
	return client, nil
}

func resolveMarketplaceConfig(cfg *coreconfig.Config, marketplaceID string) (*coreconfig.MarketplaceConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	for _, market := range cfg.Amazon.SPAPI.Marketplaces {
		if strings.EqualFold(strings.TrimSpace(market.MarketplaceID), marketplaceID) {
			m := market
			return &m, nil
		}
	}

	key := strings.TrimSpace(cfg.Amazon.SPAPI.DefaultMarketplace)
	if key != "" {
		if market, ok := cfg.Amazon.SPAPI.Marketplaces[key]; ok && strings.TrimSpace(market.MarketplaceID) != "" {
			m := market
			return &m, nil
		}
	}

	return nil, fmt.Errorf("marketplace %s is not configured", marketplaceID)
}
