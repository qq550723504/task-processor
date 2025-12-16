// Package main 提供Amazon上架流程测试工具
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"task-processor/common/utils"
	"task-processor/internal/config"
	"task-processor/platforms/amazon"

	"github.com/sirupsen/logrus"
)

var (
	productID = flag.String("product", "test-product-001", "测试产品ID")
	verbose   = flag.Bool("verbose", false, "详细日志输出")
)

func main() {
	flag.Parse()

	// 使用统一的日志设置（同时输出到控制台和文件）
	logger := utils.SetupLogger()

	// 设置日志级别
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
		logger.Info("🔧 启用详细日志模式")
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logger.Info("🚀 Amazon管道流程测试开始")

	// 加载配置
	cfg := config.LoadConfig()
	if cfg == nil {
		logger.Fatal("❌ 配置加载失败")
	}

	// 创建Amazon处理器
	processor := amazon.NewProcessor(cfg, logger)

	// 创建测试上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 启动处理器
	if err := processor.Start(ctx); err != nil {
		logger.WithError(err).Fatal("❌ 处理器启动失败")
	}
	defer processor.Stop(ctx)

	// 执行管道流程测试
	if err := runPipelineTest(ctx, processor, *productID); err != nil {
		logger.WithError(err).Fatal("❌ 管道流程测试失败")
	}

	logger.Info("✅ Amazon管道流程测试完成")
}

// runPipelineTest 运行管道流程测试
func runPipelineTest(ctx context.Context, processor *amazon.Processor, productID string) error {
	logrus.Info("📦 开始管道流程详细测试")

	// 创建测试产品数据
	testData := createTestProductData(productID)

	// 打印测试数据
	logrus.Info("📋 测试产品数据:")
	printProductData(testData)

	// 使用处理器的管道流程处理
	return processor.ProcessTaskWithPipeline(ctx, testData)
}

// createTestProductData 创建测试产品数据
func createTestProductData(productID string) map[string]interface{} {
	return map[string]interface{}{
		"product_id": productID,
		"store_id":   int64(1001),
		"tenant_id":  int64(1),
		"raw_json_data": `{
			"title": "韩版修身显瘦长袖连衣裙女装春秋新款",
			"brand": "时尚女装",
			"description": "优雅的韩版修身连衣裙，采用高品质面料，显瘦效果佳，适合春秋季节穿着。精致的剪裁和时尚的设计，让您在任何场合都能展现优雅气质。",
			"price": "199.00",
			"currency": "CNY",
			"color": "黑色",
			"size": "M",
			"material": "棉混纺",
			"category": "女装/连衣裙",
			"images": [
				"https://example.com/image1.jpg",
				"https://example.com/image2.jpg"
			],
			"variants": [
				{
					"color": "黑色",
					"size": "S",
					"price": "199.00",
					"stock": 50
				},
				{
					"color": "黑色", 
					"size": "M",
					"price": "199.00",
					"stock": 80
				},
				{
					"color": "白色",
					"size": "S", 
					"price": "199.00",
					"stock": 30
				}
			],
			"specifications": {
				"sleeve_length": "长袖",
				"neckline": "圆领",
				"style": "韩版",
				"season": "春秋"
			}
		}`,
		"source_platform": "1688",
		"target_platform": "amazon",
		"marketplace_id":  "ATVPDKIKX0DER",
		"language_tag":    "en_US",
		"currency_target": "USD",
	}
}

// printProductData 打印产品数据
func printProductData(data map[string]interface{}) {
	// 解析原始JSON数据用于美化显示
	if rawJSON, ok := data["raw_json_data"].(string); ok {
		var productData map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &productData); err == nil {
			logrus.Infof("  📝 产品标题: %v", productData["title"])
			logrus.Infof("  🏷️  产品品牌: %v", productData["brand"])
			logrus.Infof("  💰 产品价格: %v %s", productData["price"], productData["currency"])
			logrus.Infof("  🎨 产品颜色: %v", productData["color"])
			logrus.Infof("  📏 产品尺寸: %v", productData["size"])

			if variants, ok := productData["variants"].([]interface{}); ok {
				logrus.Infof("  🔄 变体数量: %d", len(variants))
			}
		}
	}

	logrus.Infof("  🆔 产品ID: %v", data["product_id"])
	logrus.Infof("  🏪 店铺ID: %v", data["store_id"])
	logrus.Infof("  🌍 目标市场: %v", data["marketplace_id"])
}

// printStatus 打印状态信息
func printStatus(status map[string]interface{}) {
	statusJSON, _ := json.MarshalIndent(status, "  ", "  ")
	fmt.Printf("  %s\n", string(statusJSON))
}
