// Package main 提供Amazon爬虫调试测试程序
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/model"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志级别为Debug以便查看详细信息
	logrus.SetLevel(logrus.DebugLevel)

	fmt.Println("=== Amazon爬虫调试测试程序 ===")
	fmt.Println("此程序专门用于调试和测试Amazon产品爬取")
	fmt.Println("特点：")
	fmt.Println("1. 详细的调试日志输出")
	fmt.Println("2. 支持自定义邮编(zipcode)")
	fmt.Println("3. 自动处理验证码和反爬")
	fmt.Println("4. 保存完整产品信息到JSON文件")
	fmt.Println()

	// 加载配置
	cfg := config.LoadConfig()

	// 创建Amazon处理器
	processor := amazon.NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	// 默认测试URL和邮编
	testURL := "https://www.amazon.com/dp/B0FHVPK4SL"
	zipcode := "10001" // 纽约邮编

	// 解析命令行参数
	if len(os.Args) > 1 {
		testURL = os.Args[1]
	}
	if len(os.Args) > 2 {
		zipcode = os.Args[2]
	}

	fmt.Printf("测试URL: %s\n", testURL)
	fmt.Printf("邮编: %s\n", zipcode)
	fmt.Println()
	fmt.Println("调试信息说明：")
	fmt.Println("- 会显示页面加载和数据提取的详细信息")
	fmt.Println("- 会显示验证码处理过程")
	fmt.Println("- 请观察浏览器中的操作是否正常")
	fmt.Println()

	fmt.Println("开始处理，请注意观察...")
	fmt.Println("如果遇到验证码，程序会尝试自动处理")
	fmt.Println()

	// 开始处理
	product, err := processor.Process(testURL, zipcode)
	if err != nil {
		log.Printf("❌ 处理产品失败: %v", err)
		return
	}

	fmt.Printf("✅ 产品爬取成功！\n")
	fmt.Printf("产品ASIN: %s\n", product.Asin)
	fmt.Printf("产品标题: %s\n", product.Title)
	fmt.Printf("原价: %.2f %s\n", product.InitialPrice, product.Currency)
	fmt.Printf("现价: %.2f %s\n", product.FinalPrice, product.Currency)
	fmt.Printf("产品评分: %.1f (%d 评价)\n", product.Rating, product.ReviewsCount)
	fmt.Printf("产品图片数量: %d\n", len(product.Images))
	if len(product.Images) > 0 {
		fmt.Printf("主图: %s\n", product.Images[0])
	}

	// 保存详细结果到JSON文件
	outputFile := fmt.Sprintf("amazon_product_%s_%s.json", product.Asin, time.Now().Format("20060102_150405"))
	if err := saveProductToFile(product, outputFile); err != nil {
		log.Printf("⚠️ 保存文件失败: %v", err)
	} else {
		fmt.Printf("\n📁 详细信息已保存到: %s\n", outputFile)
		fmt.Printf("文件包含:\n")
		fmt.Printf("  - 基础信息: 标题、价格、评分、图片\n")
		fmt.Printf("  - 商品属性: %d个\n", len(product.Features))
		fmt.Printf("  - 变体信息: %d个维度\n", len(product.Variations))
		fmt.Printf("  - 商品描述: %s\n", truncateString(product.Description, 50))
	}

	fmt.Println()
	fmt.Println("🎉 调试测试完成！Amazon爬虫工作正常。")
}

// saveProductToFile 保存产品信息到JSON文件
func saveProductToFile(product *model.Product, filename string) error {
	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 序列化为JSON（格式化输出）
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(product); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	return nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
