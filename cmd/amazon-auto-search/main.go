// Package main 搜索Amazon汽配产品类型
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
	logger.Info("=== 搜索Amazon汽配产品类型 ===")

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

	// 搜索汽配相关的产品类型
	keywords := []string{"auto", "automotive", "car", "vehicle", "part"}

	for _, keyword := range keywords {
		logger.Infof("🔍 搜索关键词: %s", keyword)

		productTypes, err := client.GetProductTypeDefinitions(ctx, []string{keyword})
		if err != nil {
			logger.Errorf("❌ 搜索失败: %v", err)
			continue
		}

		logger.Infof("📋 找到 %d 个相关产品类型:", len(productTypes))
		for i, pt := range productTypes {
			if i >= 10 { // 只显示前10个
				logger.Infof("  ... 还有 %d 个产品类型", len(productTypes)-10)
				break
			}
			logger.Infof("  %d. %s - %s", i+1, pt.Name, pt.DisplayName)
		}
		logger.Info("")
	}

	logger.Info("✅ 搜索完成")
}
