package publishing

import (
	"context"
	"fmt"
	"strings"

	amazonapi "task-processor/internal/amazon/api"
	coreconfig "task-processor/internal/core/config"
	amazonmodel "task-processor/internal/marketplace/amazon/model"
)

type SPAPISubmitter struct {
	cfg *coreconfig.Config
}

func NewSPAPISubmitter(cfg *coreconfig.Config) *SPAPISubmitter {
	if cfg == nil || !cfg.Amazon.SPAPI.Enabled {
		return nil
	}
	return &SPAPISubmitter{cfg: cfg}
}

func (s *SPAPISubmitter) Preview(ctx context.Context, export *amazonmodel.AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.ValidationPreviewRequest == nil {
		return nil, fmt.Errorf("validation preview request is empty")
	}
	return client.ValidateListing(ctx, export.ValidationPreviewRequest)
}

func (s *SPAPISubmitter) Create(ctx context.Context, export *amazonmodel.AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.CreateRequest == nil {
		return nil, fmt.Errorf("create request is empty")
	}
	return client.CreateListing(ctx, export.CreateRequest)
}

func (s *SPAPISubmitter) Update(ctx context.Context, export *amazonmodel.AmazonListingsAPIExport) (*amazonapi.ListingResponse, error) {
	client, err := s.newClient(export)
	if err != nil {
		return nil, err
	}
	if export == nil || export.UpdateRequest == nil {
		return nil, fmt.Errorf("update request is empty")
	}
	return client.UpdateListing(ctx, export.UpdateRequest)
}

func (s *SPAPISubmitter) newClient(export *amazonmodel.AmazonListingsAPIExport) (*amazonapi.Client, error) {
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
	if market := coreconfig.ResolveAmazonMarketplaceConfig(cfg.Amazon.SPAPI, marketplaceID); market != nil {
		return market, nil
	}

	return nil, fmt.Errorf("marketplace %s is not configured", marketplaceID)
}
