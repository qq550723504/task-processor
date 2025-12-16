// Package main 查询Amazon产品详细信息
package main

import (
	"context"
	"task-processor/internal/config"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Info("=== Amazon产品详细信息查询 ===")

	// 加载配置
	cfg := config.LoadConfig()

	// 创建Amazon API客户端
	amazonCfg := cfg.Amazon.SPAPI
	targetMarket := amazonCfg.DefaultMarketplace
	if targetMarket == "" {
		targetMarket = "us"
	}

	marketConfig := amazonCfg.Marketplaces[targetMarket]

	client := api.NewClient(&api.Config{
		Region:        amazonCfg.Region,
		MarketplaceID: marketConfig.MarketplaceID,
		SellerID:      marketConfig.SellerID,
		ClientID:      amazonCfg.ClientID,
		ClientSecret:  amazonCfg.ClientSecret,
		RefreshToken:  amazonCfg.RefreshToken,
	})

	// 查询产品详细信息
	sku := "XV-GKRG-7DCX"
	asin := "B0G2QLVCPB"

	logger.Infof("🔍 查询产品详细信息:")
	logger.Infof("  SKU: %s", sku)
	logger.Infof("  ASIN: %s", asin)
	logger.Infof("  MarketplaceID: %s", marketConfig.MarketplaceID)

	ctx := context.Background()

	// 获取详细的listing信息
	_, err := client.GetDetailedListing(ctx, sku, marketConfig.MarketplaceID)
	if err != nil {
		logger.Errorf("❌ 产品详细信息查询失败: %v", err)
	} else {
		logger.Info("✅ 产品详细信息查询成功")
	}

	// 获取SWEATSHIRT产品类型定义
	logger.Info("🔍 获取SWEATSHIRT产品类型定义")
	productTypes, err := client.GetProductTypeDefinitions(ctx, []string{"SWEATSHIRT"})
	if err != nil {
		logger.Errorf("❌ 获取产品类型定义失败: %v", err)
	} else {
		logger.Infof("✅ 获取到产品类型定义，共 %d 个", len(productTypes))
		for i, pt := range productTypes {
			logger.Infof("产品类型 %d:", i+1)
			logger.Infof("  名称: %s", pt.Name)
			logger.Infof("  显示名称: %s", pt.DisplayName)
			logger.Infof("  市场ID: %s", pt.MarketplaceID)
		}
	}

	logger.Info("✅ 查询完成")
}
