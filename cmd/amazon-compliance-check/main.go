// Package main 查看AUTO_PART产品类型的合规证书字段枚举值
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/config"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Info("=== 查看AUTO_PART合规证书字段枚举值 ===")

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

	// 获取AUTO_PART产品类型的详细定义
	definition, err := client.GetProductTypeDefinition(ctx, "AUTO_PART")
	if err != nil {
		logger.Errorf("❌ 获取产品类型定义失败: %v", err)
		return
	}

	// 下载并解析schema
	if definition.Schema != nil {
		if schemaMap, ok := definition.Schema.Link.(map[string]interface{}); ok {
			if resource, ok := schemaMap["resource"].(string); ok {
				logger.Infof("📋 下载Schema: %s", resource)

				schemaContent, err := client.DownloadSchema(ctx, resource)
				if err != nil {
					logger.Errorf("❌ 下载schema失败: %v", err)
					return
				}

				// 查找required_product_compliance_certificate字段
				if properties, ok := schemaContent["properties"].(map[string]interface{}); ok {
					if complianceField, exists := properties["required_product_compliance_certificate"]; exists {
						logger.Info("🔍 找到required_product_compliance_certificate字段:")

						// 美化输出JSON
						complianceJSON, _ := json.MarshalIndent(complianceField, "", "  ")
						fmt.Printf("字段定义:\n%s\n", string(complianceJSON))

						// 尝试提取枚举值
						if fieldMap, ok := complianceField.(map[string]interface{}); ok {
							if items, ok := fieldMap["items"].(map[string]interface{}); ok {
								if props, ok := items["properties"].(map[string]interface{}); ok {
									if valueField, ok := props["value"].(map[string]interface{}); ok {
										if enum, ok := valueField["enum"].([]interface{}); ok {
											logger.Info("✅ 允许的枚举值:")
											for i, val := range enum {
												logger.Infof("  %d. %v", i+1, val)
											}
										}

										if enumNames, ok := valueField["enumNames"].([]interface{}); ok {
											logger.Info("📝 枚举值说明:")
											for i, name := range enumNames {
												logger.Infof("  %d. %v", i+1, name)
											}
										}
									}
								}
							}
						}
					} else {
						logger.Warn("⚠️  未找到required_product_compliance_certificate字段")
					}
				}
			}
		}
	}

	logger.Info("✅ 查看完成")
}
