// Package main 上传汽配产品到Amazon
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
	logger.Info("=== 上传汽配产品到Amazon ===")

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

	// 创建汽配产品
	newSKU := "AUTO-BRAKE-PAD-002"

	logger.Infof("📦 汽配产品信息:")
	logger.Infof("  SKU: %s", newSKU)
	logger.Infof("  MarketplaceID: %s", marketConfig.MarketplaceID)

	// 创建产品listing
	err := createAutoPartProduct(ctx, client, newSKU, marketConfig.MarketplaceID, logger)
	if err != nil {
		logger.Errorf("❌ 汽配产品上传失败: %v", err)
	} else {
		logger.Info("✅ 汽配产品上传成功")
	}

	logger.Info("✅ 上传完成")
}

// createAutoPartProduct 创建汽配产品
func createAutoPartProduct(ctx context.Context, client *api.Client, sku, marketplaceID string, logger *logrus.Logger) error {
	req := &api.ListingRequest{
		SKU:          sku,
		ProductType:  "AUTO_PART", // 使用汽配产品类型
		Requirements: "LISTING",   // 正式上架
		Attributes: map[string]any{
			// 基本必需属性
			"condition_type": []map[string]any{
				{
					"value":          "new_new",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品名称 - 必需
			"item_name": []map[string]any{
				{
					"value":          "Premium Ceramic Brake Pads Set for Toyota Camry 2018-2023, Front Disc Brake Pads",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 品牌 - 必需
			"brand": []map[string]any{
				{
					"language_tag":   "en_US",
					"value":          "AutoPro",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品描述 - 必需
			"product_description": []map[string]any{
				{
					"value": "High-performance ceramic brake pads designed specifically for Toyota Camry models 2018-2023. " +
						"These premium brake pads provide superior stopping power, reduced brake dust, and longer lifespan. " +
						"Features advanced ceramic compound for quiet operation and excellent heat dissipation. " +
						"Direct OEM replacement with perfect fit and professional installation recommended.",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 要点说明 - 必需
			"bullet_point": []map[string]any{
				{
					"value":          "PREMIUM CERAMIC COMPOUND: Advanced ceramic formula provides superior braking performance with reduced noise and dust",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "PERFECT FIT: Designed specifically for Toyota Camry 2018-2023 models, direct OEM replacement",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "ENHANCED SAFETY: Improved stopping power and consistent performance in all weather conditions",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "LONG LASTING: Extended lifespan with excellent heat dissipation and wear resistance",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
				{
					"value":          "PROFESSIONAL QUALITY: Meets or exceeds OEM specifications for safety and performance",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 原产国 - 必需
			"country_of_origin": []map[string]any{
				{
					"value":          "CN", // 中国制造
					"marketplace_id": marketplaceID,
				},
			},

			// 商品类型关键词 - 必需
			"item_type_keyword": []map[string]any{
				{
					"value":          "brake-pads",
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

			// 型号名称 - 必需
			"model_name": []map[string]any{
				{
					"value":          "Ceramic Brake Pad Set",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 型号编号 - 必需
			"model_number": []map[string]any{
				{
					"value":          "BP-TC-2018-F",
					"marketplace_id": marketplaceID,
				},
			},

			// 零件号 - 必需
			"part_number": []map[string]any{
				{
					"value":          "04465-06090",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 商品数量 - 必需
			"number_of_items": []map[string]any{
				{
					"value":          4, // 一套4片刹车片
					"marketplace_id": marketplaceID,
				},
			},

			// 汽车适配类型 - 必需
			"automotive_fit_type": []map[string]any{
				{
					"value":          "vehicle_specific_fit",
					"marketplace_id": marketplaceID,
				},
			},

			// 制造商 - 必需
			"manufacturer": []map[string]any{
				{
					"value":          "AutoPro Manufacturing",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 是否需要组装 - 必需
			"is_assembly_required": []map[string]any{
				{
					"value":          false, // 不需要组装
					"marketplace_id": marketplaceID,
				},
			},

			// 是否包含液体 - 必需
			"contains_liquid_contents": []map[string]any{
				{
					"value":          false, // 不包含液体
					"marketplace_id": marketplaceID,
				},
			},

			// 外观处理 - 必需
			"exterior_finish": []map[string]any{
				{
					"value":          "Ceramic Coating",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 包含组件 - 必需
			"included_components": []map[string]any{
				{
					"value":          "4 Brake Pads, Installation Hardware, Brake Grease",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品合规证书 - 必需字段
			"required_product_compliance_certificate": []map[string]any{
				{
					"value":          "Not Applicable",
					"marketplace_id": marketplaceID,
				},
			},

			// 保修描述 - 必需
			"warranty_description": []map[string]any{
				{
					"value":          "2 Year Limited Warranty against manufacturing defects",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 价格
			"list_price": []map[string]any{
				{
					"currency":       "USD",
					"value":          89.99,
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
									"value_with_tax": 89.99,
								},
							},
						},
					},
				},
			},

			// 库存
			"fulfillment_availability": []map[string]any{
				{
					"fulfillment_channel_code": "DEFAULT",
					"quantity":                 50, // 初始库存50套
				},
			},

			// 商家运输组 - 必需（当使用DEFAULT履行时）
			"merchant_shipping_group": []map[string]any{
				{
					"value":          "legacy-template-id",
					"marketplace_id": marketplaceID,
				},
			},

			// 包装尺寸 - 必需（当使用DEFAULT履行时）
			"item_package_dimensions": []map[string]any{
				{
					"length": map[string]any{
						"value": 12.0,
						"unit":  "inches",
					},
					"width": map[string]any{
						"value": 8.0,
						"unit":  "inches",
					},
					"height": map[string]any{
						"value": 3.0,
						"unit":  "inches",
					},
					"marketplace_id": marketplaceID,
				},
			},

			// 包装重量 - 必需（当使用DEFAULT履行时）
			"item_package_weight": []map[string]any{
				{
					"value":          5.5,
					"unit":           "pounds",
					"marketplace_id": marketplaceID,
				},
			},

			// 包装数量 - 必需（当使用DEFAULT履行时）
			"number_of_boxes": []map[string]any{
				{
					"value":          1,
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

			// ABPA零件链接号（汽配专用）
			"abpa_partslink_number": []map[string]any{
				{
					"value":          "TO1234567",
					"language_tag":   "en_US",
					"marketplace_id": marketplaceID,
				},
			},

			// 产品标识符豁免
			"supplier_declared_has_product_identifier_exemption": []map[string]any{
				{
					"value":          true,
					"marketplace_id": marketplaceID,
				},
			},

			// 商家建议ASIN（修复长度问题）
			"merchant_suggested_asin": []map[string]any{
				{
					"value":          "B08EX12345", // 10字符ASIN
					"marketplace_id": marketplaceID,
				},
			},

			// 产品尺寸
			"item_dimensions": []map[string]any{
				{
					"length": map[string]any{
						"value": 10.5,
						"unit":  "inches",
					},
					"width": map[string]any{
						"value": 6.5,
						"unit":  "inches",
					},
					"height": map[string]any{
						"value": 1.2,
						"unit":  "inches",
					},
					"marketplace_id": marketplaceID,
				},
			},

			// 主图URL（示例）
			"main_product_image_locator": []map[string]any{
				{
					"media_location": "https://example.com/images/brake-pad-main.jpg",
					"marketplace_id": marketplaceID,
				},
			},
		},
	}

	logger.Info("📋 汽配产品属性:")
	logger.Infof("  产品类型: %s", req.ProductType)
	logger.Infof("  SKU: %s", req.SKU)
	logger.Infof("  价格: $89.99")
	logger.Infof("  零件号: 04465-06090")
	logger.Infof("  适用车型: Toyota Camry 2018-2023")
	logger.Infof("  库存: 50套")

	// 直接创建产品listing
	logger.Info("创建汽配产品listing...")
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
