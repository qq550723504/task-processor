// Package main 查看Amazon产品类型Schema
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
	logger.Info("=== 查看Amazon产品类型Schema ===")

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

	ctx := context.Background()

	// 查看SWEATSHIRT产品类型的schema
	logger.Info("🔍 获取SWEATSHIRT产品类型的完整schema定义")

	err := client.AnalyzeProductTypeSchema(ctx, "SWEATSHIRT")
	if err != nil {
		logger.Errorf("❌ 获取schema失败: %v", err)
		return
	}

	logger.Info("✅ Schema分析完成")
}
