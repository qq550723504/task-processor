// Package main 提供 1688 验证码调试测试程序。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	fmt.Println("=== 1688验证码调试测试程序 ===")
	fmt.Println("此程序专门用于调试和优化滑动验证码处理")
	fmt.Println("特点：")
	fmt.Println("1. 详细的调试日志输出")
	fmt.Println("2. 优化的滑动距离计算（针对阿里系验证码）")
	fmt.Println("3. 更精细的人类行为模拟")
	fmt.Println("4. 复杂的缓动函数和轨迹控制")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	fmt.Println("=== 配置信息 ===")
	fmt.Printf("Headless: %v\n", cfg.Browser.Headless)
	fmt.Printf("BrowserPath: %s\n", cfg.Browser.BrowserPath)
	fmt.Printf("Viewport: %dx%d\n", cfg.Browser.ViewportWidth, cfg.Browser.ViewportHeight)
	fmt.Println()

	fmt.Println("强制设置 Headless 为 false...")
	cfg.Browser.Headless = false
	// 使用临时用户数据目录，确保测试是干净的
	cfg.Browser.UserDataDir = "./.local/tmp/browser-profiles/test-1688-tmp"
	fmt.Printf("修改后 Headless: %v\n", cfg.Browser.Headless)
	fmt.Printf("使用临时用户数据目录: %s\n", cfg.Browser.UserDataDir)
	fmt.Println()

	processor := alibaba1688.NewAlibaba1688Processor(cfg)
	defer processor.Shutdown()

	testURL := "https://detail.1688.com/offer/722899324071.html"
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

	product, err := processor.Process(testURL)
	if err != nil {
		log.Printf("处理产品失败: %v", err)
		return
	}

	fmt.Printf("验证码处理成功\n")
	fmt.Printf("产品ID: %s\n", product.ID)
	fmt.Printf("产品标题: %s\n", product.Title)
	fmt.Printf("产品价格: %.2f - %.2f %s\n", product.MinPrice, product.MaxPrice, product.Currency)
	fmt.Printf("产品图片: %v\n", product.Images)
	fmt.Printf("供应商: %s\n", product.Supplier.Name)

	outputFile := fmt.Sprintf("1688_product_%s_%s.json", product.ID, time.Now().Format("20060102_150405"))
	if err := saveProductToFile(product, outputFile); err != nil {
		log.Printf("保存文件失败: %v", err)
	} else {
		fmt.Printf("\n详细信息已保存到: %s\n", outputFile)
		fmt.Printf("文件包含:\n")
		fmt.Printf("  - 基础信息: 标题、价格、图片、供应商\n")
		fmt.Printf("  - 商品属性: %d个\n", len(product.Specifications))
		fmt.Printf("  - 变体值: %d个\n", len(product.VariationsValues))
		fmt.Printf("  - 商品详情: %d个部分\n", len(product.ProductDetails))
	}
}

func saveProductToFile(product *model.Product1688, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(product); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	return nil
}
