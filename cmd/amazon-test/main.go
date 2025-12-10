// Package main 提供Amazon商品上传测试程序
package main

import (
	"context"
	"flag"
	"task-processor/common/config"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

func main() {
	// 命令行参数
	sku := flag.String("sku", "TEST-SKU-001", "产品SKU")
	title := flag.String("title", "测试商品", "商品标题")
	brand := flag.String("brand", "TestBrand", "品牌名称")
	price := flag.Float64("price", 19.99, "商品价格")
	debug := flag.Bool("debug", false, "启用调试模式")
	flag.Parse()

	// 初始化日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger.Info("=== Amazon 商品上传测试 ===")

	// 加载配置
	cfg := config.LoadConfig()
	if cfg == nil {
		logger.Fatal("❌ 加载配置失败")
	}

	// 调试模式
	if *debug {
		debugConfig(cfg, logger)
		return
	}

	// 检查配置
	if err := checkConfig(cfg, logger); err != nil {
		logger.Fatalf("❌ 配置检查失败: %v", err)
	}

	// 打印配置信息
	printConfig(cfg, logger)

	// 检查是否启用
	if !cfg.Amazon.SPAPI.Enabled {
		logger.Warn("⚠️  Amazon SP-API 未启用，但继续测试...")
	}

	// 创建API客户端
	apiClient := api.NewClient(&api.Config{
		Region:        cfg.Amazon.SPAPI.Region,
		MarketplaceID: cfg.Amazon.SPAPI.MarketplaceID,
		ClientID:      cfg.Amazon.SPAPI.ClientID,
		ClientSecret:  cfg.Amazon.SPAPI.ClientSecret,
		RefreshToken:  cfg.Amazon.SPAPI.RefreshToken,
		Sandbox:       cfg.Amazon.SPAPI.Sandbox,
	})

	logger.Info("✅ API客户端创建成功")
	if cfg.Amazon.SPAPI.Sandbox {
		logger.Warn("⚠️  沙盒模式：所有操作仅用于测试，不会影响真实数据")
	}

	// 创建上下文
	ctx := context.Background()

	// 测试认证
	logger.Info("🔐 测试认证...")
	token, err := apiClient.GetAccessToken(ctx)
	if err != nil {
		logger.Fatalf("❌ 认证失败: %v", err)
	}
	logger.Infof("✅ 认证成功，Token: %s...", token[:20])

	// 构建商品数据
	logger.Info("📦 准备商品数据...")
	listingReq := &api.ListingRequest{
		SKU:          *sku,
		ProductType:  "PRODUCT",
		Requirements: "LISTING",
		Attributes: map[string]interface{}{
			"item_name": []map[string]string{
				{
					"value":          *title,
					"language_tag":   "en_US",
					"marketplace_id": cfg.Amazon.SPAPI.MarketplaceID,
				},
			},
			"brand": []map[string]string{
				{
					"value":          *brand,
					"language_tag":   "en_US",
					"marketplace_id": cfg.Amazon.SPAPI.MarketplaceID,
				},
			},
			"manufacturer": []map[string]string{
				{
					"value":          *brand,
					"language_tag":   "en_US",
					"marketplace_id": cfg.Amazon.SPAPI.MarketplaceID,
				},
			},
			"condition_type": []map[string]string{
				{
					"value":          "new_new",
					"marketplace_id": cfg.Amazon.SPAPI.MarketplaceID,
				},
			},
			"purchasable_offer": []map[string]interface{}{
				{
					"marketplace_id": cfg.Amazon.SPAPI.MarketplaceID,
					"currency":       "USD",
					"our_price": []map[string]interface{}{
						{
							"schedule": []map[string]interface{}{
								{
									"value_with_tax": *price,
								},
							},
						},
					},
				},
			},
		},
	}

	logger.Infof("   SKU: %s", *sku)
	logger.Infof("   标题: %s", *title)
	logger.Infof("   品牌: %s", *brand)
	logger.Infof("   价格: $%.2f", *price)

	// 创建Listing
	logger.Info("🚀 开始上传商品...")
	resp, err := apiClient.CreateListing(ctx, listingReq)
	if err != nil {
		logger.Fatalf("❌ 上传失败: %v", err)
	}

	// 输出结果
	logger.Info("✅ 商品上传成功！")
	logger.Infof("   SKU: %s", resp.SKU)
	logger.Infof("   状态: %s", resp.Status)

	if len(resp.Issues) > 0 {
		logger.Warn("⚠️  存在以下问题:")
		for i, issue := range resp.Issues {
			logger.Warnf("   %d. [%s] %s: %s", i+1, issue.Severity, issue.Code, issue.Message)
		}
	}

	logger.Info("=== 测试完成 ===")
}
