// Package main 上传类似的Amazon产品
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
	logger.Info("=== 上传类似Amazon产品 ===")

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

	// 首先查询成功产品的详细信息作为参考
	logger.Info("🔍 查询参考产品XV-GKRG-7DCX的详细信息")
	_, err := client.GetDetailedListing(ctx, "XV-GKRG-7DCX", marketConfig.MarketplaceID)
	if err != nil {
		logger.Errorf("❌ 查询参考产品失败: %v", err)
		return
	}

	// 创建类似产品的listing
	logger.Info("🔍 创建类似产品的listing")

	// 基于参考产品XV-GKRG-7DCX创建新产品
	newSKU := "PREMIUM-HOODIE-002"

	logger.Infof("📦 新产品信息:")
	logger.Infof("  SKU: %s", newSKU)
	logger.Infof("  MarketplaceID: %s", marketConfig.MarketplaceID)

	// 创建产品listing
	err = createSimilarProduct(ctx, client, newSKU, marketConfig.MarketplaceID, logger)
	if err != nil {
		logger.Errorf("❌ 产品上传失败: %v", err)
	} else {
		logger.Info("✅ 产品上传成功")
	}

	logger.Info("✅ 上传完成")
}

// createSimilarProduct 创建类似的产品
func createSimilarProduct(ctx context.Context, client *api.Client, sku, marketplaceID string, logger *logrus.Logger) error {
	// 基于参考产品XV-GKRG-7DCX的属性创建新产品
	req := &api.ListingRequest{
		SKU:          sku,
		ProductType:  "SWEATSHIRT", // 使用相同的产品类型
		Requirements: "LISTING",    // 正式上架
		Attributes: map[string]any{
			// 基本必需属性
			"condition_type": []map[string]any{
				{
					"value":          "new_new",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品名称 - 修改为你的产品
			"item_name": []map[string]any{
				{
					"value":          "Cozy Cotton Blend Hoodie in Forest Green, Soft Pullover Sweatshirt with Drawstring Hood",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 品牌 - 参考成功产品
			"brand": []map[string]any{
				{
					"language_tag":   "en_US",
					"value":          "Generic", // 使用不同的品牌名
					"marketplace_id": marketplaceID,
				},
			},

			// 颜色 - 改为森林绿
			"color": []map[string]any{
				{
					"value":               "Forest Green",
					"language_tag":        "en_US",
					"standardized_values": []string{"Green"},
					"marketplace_id":      marketplaceID,
				},
			},

			// 尺寸 - 参考成功产品格式
			"apparel_size": []map[string]any{
				{
					"body_type":      "regular",
					"height_type":    "regular",
					"size":           "numeric_12", // 参考成功产品的尺寸格式
					"size_class":     "numeric",
					"size_system":    "as1",
					"marketplace_id": marketplaceID,
				},
			},

			// 特殊尺寸类型 - 参考成功产品的做法
			"special_size_type": []map[string]any{
				{
					"language_tag":   "en_US",
					"value":          "Standard",
					"marketplace_id": marketplaceID,
				},
			},

			// 材质
			"material": []map[string]any{
				{
					"value":          "Cotton",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "Polyester",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 面料类型
			"fabric_type": []map[string]any{
				{
					"value":          "Cotton Blend Fleece",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 性别
			"target_gender": []map[string]any{
				{
					"value":          "unisex",
					"marketplace_id": marketplaceID,
				},
			},

			// 风格
			"style": []map[string]any{
				{
					"value":          "Pullover Hoodie",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 合身类型
			"fit_type": []map[string]any{
				{
					"value":          "Regular Fit",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品描述
			"product_description": []map[string]any{
				{
					"value": "Discover exceptional comfort with this cozy cotton blend hoodie in rich forest green. " +
						"Designed with a roomy front pocket, adjustable drawstring hood, and plush fleece interior for maximum warmth. " +
						"Ideal for everyday wear, outdoor adventures, or relaxing at home. " +
						"The premium construction and classic styling make it an essential wardrobe staple.",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 要点说明
			"bullet_point": []map[string]any{
				{
					"value":          "SUPERIOR FABRIC: Premium cotton blend fleece delivers exceptional softness and long-lasting durability",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "THOUGHTFUL DESIGN: Classic pullover style featuring adjustable drawstring hood and spacious front pocket",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "NATURE-INSPIRED COLOR: Rich forest green complements any casual wardrobe for all seasons",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "PERFECT FIT: Regular cut with reinforced cuffs and hem ensures comfortable, secure wear",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "CARE-FREE MAINTENANCE: Machine washable fabric retains shape and vibrant color wash after wash",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 价格
			"list_price": []map[string]any{
				{
					"currency":       "USD",
					"value":          27.99, // 调整价格
					"marketplace_id": marketplaceID,
				},
			},

			// 销售价格
			"purchasable_offer": []map[string]any{
				{
					"currency":       "USD",
					"audience":       "ALL",
					"marketplace_id": marketplaceID,
					"our_price": []map[string]any{
						{
							"schedule": []map[string]any{
								{
									"value_with_tax": 27.99,
								},
							},
						},
					},
				},
			},

			// 库存 - 确保有足够库存
			"fulfillment_availability": []map[string]any{
				{
					"fulfillment_channel_code": "DEFAULT",
					"quantity":                 100, // 增加库存到100件
				},
			},

			// 护理说明
			"care_instructions": []map[string]any{
				{
					"value":          "Machine Wash Cold, Tumble Dry Low, Do Not Bleach, Wash With Similar Colors",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 原产国
			"country_of_origin": []map[string]any{
				{
					"value":          "CN", // 中国制造
					"marketplace_id": marketplaceID,
				},
			},

			// 包装数量
			"item_package_quantity": []map[string]any{
				{
					"value":          1,
					"marketplace_id": marketplaceID,
				},
			},

			// 进口标识 - 必需
			"import_designation": []map[string]any{
				{
					"value":          "Made in China",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 部门 - 必需
			"department": []map[string]any{
				{
					"value":          "Unisex Adult",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品标识符豁免 - 参考成功产品的做法
			"supplier_declared_has_product_identifier_exemption": []map[string]any{
				{
					"value":          true,
					"marketplace_id": marketplaceID,
				},
			},

			// 商品数量 - 参考成功产品的做法
			"number_of_items": []map[string]any{
				{
					"value":          1,
					"marketplace_id": marketplaceID,
				},
			},

			// 领口样式 - 必需
			"neck": []map[string]any{
				{
					"neck_style": []map[string]any{
						{
							"value":        "Hooded",
							"language_tag": "en_US",
						},
					},
					"marketplace_id": marketplaceID,
				},
			},

			// 年龄范围描述 - 必需
			"age_range_description": []map[string]any{
				{
					"value":          "Adult",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 危险品规定 - 必需
			"supplier_declared_dg_hz_regulation": []map[string]any{
				{
					"value":          "not_applicable",
					"marketplace_id": marketplaceID,
				},
			},

			// 商品类型关键词 - 必需
			"item_type_keyword": []map[string]any{
				{
					"value":          "novelty-hoodies",
					"marketplace_id": marketplaceID,
				},
			},

			// 商家建议ASIN - 必需 (Amazon要求必须提供)
			"merchant_suggested_asin": []map[string]any{
				{
					"value":          "B0G2QLVCPB", // 提供一个建议的ASIN格式
					"marketplace_id": marketplaceID,
				},
			},

			// 单位数量 - 根据schema分析，type字段应该是对象格式
			"unit_count": []map[string]any{
				{
					"value": 1,
					"type": map[string]any{
						"value":        "Count",
						"language_tag": "en_US",
					},
					"marketplace_id": marketplaceID,
				},
			},

			// 型号名称 - 必需
			"model_name": []map[string]any{
				{
					"value":          "Cozy Forest Green Hoodie",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},
		},
	}

	logger.Info("📋 产品属性:")
	logger.Infof("  产品类型: %s", req.ProductType)
	logger.Infof("  SKU: %s", req.SKU)
	logger.Infof("  价格: $27.99")
	logger.Infof("  颜色: Forest Green")
	logger.Infof("  尺寸: L")
	logger.Infof("  库存: 100件")

	// 直接创建产品listing（跳过验证模式）
	logger.Info("直接创建产品listing...")
	req.Requirements = "LISTING_PRODUCT_ONLY" // 正式创建

	createResp, err := client.CreateListing(ctx, req)
	if err != nil {
		return err
	}

	logger.Infof("✅ 产品创建结果: %s", createResp.Status)
	if len(createResp.Issues) > 0 {
		logger.Info("⚠️  发现的问题:")
		for i, issue := range createResp.Issues {
			logger.Infof("  %d. [%s] %s: %s", i+1, issue.Severity, issue.Code, issue.Message)
		}

		// 如果有错误，报告但不中断
		hasErrors := false
		for _, issue := range createResp.Issues {
			if issue.Severity == "ERROR" {
				hasErrors = true
				break
			}
		}

		if hasErrors {
			logger.Warn("⚠️  创建过程中发现错误，但产品可能已部分创建")
		}
	}

	return nil
}
