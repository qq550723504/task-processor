// Package main 提供 productenrich 商品信息增强调试测试程序
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	fmt.Println("=== productenrich 商品信息增强测试程序 ===")
	fmt.Println("此程序用于调试和测试商品信息增强（LLM生成）流程")
	fmt.Println("特点：")
	fmt.Println("1. 同步执行，无需启动 Worker Pool")
	fmt.Println("2. 全内存存储，无需数据库/Redis")
	fmt.Println("3. 支持三种输入模式：图片URL、文本描述、1688商品URL")
	fmt.Println()

	cfg := config.LoadConfig()

	// 默认测试数据：1688 商品 URL
	req := &productenrich.GenerateRequest{
		ProductURL: "https://detail.1688.com/offer/722899324071.html",
	}

	// 解析命令行参数
	// 用法：
	//   ./test-productenrich                          # 使用默认1688 URL
	//   ./test-productenrich <url>                    # 指定商品URL或文本
	//   ./test-productenrich <image_url> <text>       # 图片URL + 文本描述
	switch len(os.Args) {
	case 2:
		arg := os.Args[1]
		if len(arg) >= 4 && arg[:4] == "http" {
			req = &productenrich.GenerateRequest{ProductURL: arg}
		} else {
			req = &productenrich.GenerateRequest{Text: arg}
		}
	case 3:
		req = &productenrich.GenerateRequest{
			ImageURLs: []string{os.Args[1]},
			Text:      os.Args[2],
		}
	}

	fmt.Printf("输入模式: %s\n", describeRequest(req))
	fmt.Println()

	svc, err := buildService(cfg)
	if err != nil {
		log.Fatalf("❌ 初始化服务失败: %v", err)
	}

	ctx := context.Background()

	// 创建任务（不设置 workerPool，所以不会自动提交，只存入内存 repo）
	task, err := svc.CreateGenerateTask(ctx, req)
	if err != nil {
		log.Fatalf("❌ 创建任务失败: %v", err)
	}
	fmt.Printf("任务ID: %s\n", task.ID)
	fmt.Println("开始处理，请稍候（LLM 调用可能需要数十秒）...")
	fmt.Println()

	start := time.Now()
	result, err := svc.ProcessProduct(ctx, task)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("❌ 处理失败 (耗时 %.1fs): %v", elapsed.Seconds(), err)
		return
	}

	fmt.Printf("✅ 处理成功！(耗时 %.1fs)\n", elapsed.Seconds())
	fmt.Printf("标题: %s\n", result.Title)
	fmt.Printf("分类: %v\n", result.Category)
	fmt.Printf("属性数量: %d\n", len(result.Attributes))
	fmt.Printf("卖点数量: %d\n", len(result.SellingPoints))
	fmt.Printf("SEO关键词: %d 个\n", len(result.SEOKeywords))
	fmt.Printf("变体数量: %d\n", len(result.Variants))
	fmt.Printf("图片数量: %d\n", len(result.Images))
	if result.Description != "" {
		desc := result.Description
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:80]) + "..."
		}
		fmt.Printf("描述摘要: %s\n", desc)
	}

	outputFile := fmt.Sprintf("productenrich_result_%s.json", time.Now().Format("20060102_150405"))
	if err := saveToFile(result, outputFile); err != nil {
		log.Printf("⚠️ 保存文件失败: %v", err)
	} else {
		fmt.Printf("\n📁 完整结果已保存到: %s\n", outputFile)
	}

	fmt.Println()
	fmt.Println("🎉 测试完成！")
}

func describeRequest(req *productenrich.GenerateRequest) string {
	switch {
	case req.ProductURL != "":
		return fmt.Sprintf("商品URL (%s)", req.ProductURL)
	case len(req.ImageURLs) > 0 && req.Text != "":
		return fmt.Sprintf("图片(%d张) + 文本", len(req.ImageURLs))
	case len(req.ImageURLs) > 0:
		return fmt.Sprintf("图片(%d张)", len(req.ImageURLs))
	default:
		return fmt.Sprintf("文本描述 (%d字符)", len(req.Text))
	}
}

func saveToFile(result *productenrich.ProductJSON, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}
	return nil
}
