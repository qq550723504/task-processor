// Package main 提供1688验证码调试测试程序
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志级别为Debug以便查看详细信息
	logrus.SetLevel(logrus.DebugLevel)

	fmt.Println("=== 1688验证码调试测试程序 ===")
	fmt.Println("此程序专门用于调试和优化滑动验证码处理")
	fmt.Println("特点：")
	fmt.Println("1. 详细的调试日志输出")
	fmt.Println("2. 优化的滑动距离计算（针对阿里系验证码）")
	fmt.Println("3. 更精细的人类行为模拟")
	fmt.Println("4. 复杂的缓动函数和轨迹控制")
	fmt.Println()

	// 创建1688配置
	cfg := &config.Alibaba1688Config{
		Enabled:  true,
		Timeout:  300, // 5分钟超时
		PoolSize: 1,
		BrowserConfig: config.BrowserConfig{
			Enabled:        true,
			Headless:       false, // 必须为false以便观察
			BrowserPath:    "./chrome/chrome.exe",
			ViewportWidth:  1920,
			ViewportHeight: 1080,
		},
		RandomConfig: config.BrowserRandomConfig{
			Enabled:            false,
			Strategy:           "stable",
			HealthCheckEnabled: false,
			MaxRetries:         3,
		},
	}

	// 创建1688处理器
	processor := alibaba1688.NewAlibaba1688Processor(cfg)
	defer processor.Shutdown()

	// 测试URL
	testURL := "https://detail.1688.com/offer/722899324071.html"

	// 如果提供了命令行参数，使用参数中的URL
	if len(os.Args) > 1 {
		testURL = os.Args[1]
	}

	fmt.Printf("测试URL: %s\n", testURL)
	fmt.Println()
	fmt.Println("调试信息说明：")
	fmt.Println("- 会显示轨道检测和距离计算的详细信息")
	fmt.Println("- 会显示滑动过程的坐标和时间信息")
	fmt.Println("- 请观察浏览器中的滑动轨迹是否自然")
	fmt.Println()

	fmt.Println("开始处理，请注意观察...")
	fmt.Println("如果自动滑动失败，请手动完成验证码")
	fmt.Println()

	// 开始处理
	product, err := processor.Process(testURL)
	if err != nil {
		log.Printf("❌ 处理产品失败: %v", err)
		return
	}

	fmt.Printf("✅ 验证码处理成功！\n")
	fmt.Printf("产品ID: %s\n", product.ID)
	fmt.Printf("产品标题: %s\n", product.Title)
	fmt.Printf("产品价格: %.2f - %.2f %s\n", product.MinPrice, product.MaxPrice, product.Currency)
	fmt.Printf("产品图片: %v\n", product.Images)
	fmt.Printf("供应商: %s\n", product.Supplier.Name)

	// 保存详细结果到JSON文件
	outputFile := fmt.Sprintf("1688_product_%s_%s.json", product.ID, time.Now().Format("20060102_150405"))
	if err := saveProductToFile(product, outputFile); err != nil {
		log.Printf("⚠️ 保存文件失败: %v", err)
	} else {
		fmt.Printf("\n📁 详细信息已保存到: %s\n", outputFile)
		fmt.Printf("文件包含:\n")
		fmt.Printf("  - 基础信息: 标题、价格、图片、供应商\n")
		fmt.Printf("  - 商品属性: %d个\n", len(product.Specifications))
		fmt.Printf("  - 变体值: %d个\n", len(product.VariationsValues))
		fmt.Printf("  - 商品详情: %d个部分\n", len(product.ProductDetails))
	}

	fmt.Println()
	fmt.Println("🎉 调试测试完成！验证码处理算法工作正常。")
}

// saveProductToFile 保存产品信息到JSON文件
func saveProductToFile(product *model.Product1688, filename string) error {
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
