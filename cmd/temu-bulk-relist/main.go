package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	// 命令行参数
	var (
		tenantID          = flag.Int64("tenant", 0, "租户ID（可从配置文件读取）")
		storeID           = flag.Int64("store", 0, "单个店铺ID（与store-ids互斥）")
		storeIDs          = flag.String("store-ids", "", "多个店铺ID（逗号分隔，如：627,628,629）")
		storeIndex        = flag.Int("store-index", 0, "使用配置文件中第几个店铺ID（从0开始）")
		allStores         = flag.Bool("all-stores", false, "处理配置文件中的所有店铺")
		delay             = flag.Int("delay", 1000, "请求间隔（毫秒）")
		concurrency       = flag.Int("concurrency", 1, "并发数量（1=串行，>1=并发）")
		skipRectify       = flag.Bool("skip-rectify", true, "跳过需要整改的商品")
		skipPunished      = flag.Bool("skip-punished", true, "跳过被严重惩罚的商品")
		skipLocked        = flag.Bool("skip-locked", true, "跳过被锁定的商品（推荐启用）")
		skipNoStock       = flag.Bool("skip-no-stock", false, "跳过无库存的商品")
		minStock          = flag.Int("min-stock", 0, "最小库存要求")
		includeCategories = flag.String("include-categories", "", "包含的分类（逗号分隔）")
		excludeCategories = flag.String("exclude-categories", "", "排除的分类（逗号分隔）")
		nameKeywords      = flag.String("name-keywords", "", "商品名称关键词（逗号分隔）")
		minPrice          = flag.Float64("min-price", 0, "最小价格")
		maxPrice          = flag.Float64("max-price", 0, "最大价格")
		dryRun            = flag.Bool("dry-run", false, "试运行模式（仅显示会处理的商品，不实际上架）")
		outputFile        = flag.String("output", "", "结果输出文件路径（JSON格式）")
		logFile           = flag.String("log-file", "", "日志输出文件路径（如：relist.log）")
		verbose           = flag.Bool("verbose", false, "详细日志输出")
		listStores        = flag.Bool("list-stores", false, "列出配置文件中的所有店铺ID")
		firstPageOnly     = flag.Bool("first-page-only", false, "只循环处理第一页（推荐，避免数据变化问题）")
	)
	flag.Parse()

	// 设置日志级别
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// 设置日志输出到文件
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("错误: 无法创建日志文件 %s: %v\n", *logFile, err)
			os.Exit(1)
		}
		defer file.Close()

		// 同时输出到控制台和文件
		logrus.SetOutput(io.MultiWriter(os.Stdout, file))
		fmt.Printf("日志将同时输出到控制台和文件: %s\n", *logFile)
	}

	// 加载配置
	cfg := config.LoadConfig()
	if cfg == nil {
		fmt.Println("错误: 无法加载配置文件")
		os.Exit(1)
	}

	// 如果用户要求列出店铺ID，则显示并退出
	if *listStores {
		fmt.Printf("配置文件中的店铺ID列表:\n")
		if len(cfg.Management.StoreIDs) == 0 {
			fmt.Printf("  (无店铺ID配置)\n")
		} else {
			for i, storeID := range cfg.Management.StoreIDs {
				fmt.Printf("  [%d] %d\n", i, storeID)
			}
		}
		fmt.Printf("租户ID: %s\n", cfg.Management.TenantID)
		os.Exit(0)
	}

	// 参数校验和配置处理
	if *tenantID == 0 {
		// 尝试从配置文件获取
		if cfg.Management.TenantID != "" {
			if tid, err := strconv.ParseInt(cfg.Management.TenantID, 10, 64); err == nil {
				*tenantID = tid
				fmt.Printf("从配置文件读取租户ID: %d\n", *tenantID)
			}
		}

		if *tenantID == 0 {
			fmt.Println("错误: 必须指定租户ID (-tenant) 或在配置文件中设置")
			flag.Usage()
			os.Exit(1)
		}
	}

	// 解析店铺ID列表
	var targetStoreIDs []int64

	if *allStores {
		// 使用配置文件中的所有店铺
		targetStoreIDs = cfg.Management.StoreIDs
		fmt.Printf("使用配置文件中的所有店铺: %v\n", targetStoreIDs)
	} else if *storeIDs != "" {
		// 解析命令行指定的多个店铺ID
		storeIDStrings := strings.Split(*storeIDs, ",")
		for _, idStr := range storeIDStrings {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				targetStoreIDs = append(targetStoreIDs, id)
			} else {
				fmt.Printf("错误: 无效的店铺ID '%s'\n", idStr)
				os.Exit(1)
			}
		}
		fmt.Printf("使用指定的店铺ID: %v\n", targetStoreIDs)
	} else if *storeID != 0 {
		// 使用单个店铺ID
		targetStoreIDs = []int64{*storeID}
		fmt.Printf("使用单个店铺ID: %d\n", *storeID)
	} else {
		// 根据storeIndex选择店铺ID
		if len(cfg.Management.StoreIDs) > 0 {
			if *storeIndex >= 0 && *storeIndex < len(cfg.Management.StoreIDs) {
				targetStoreIDs = []int64{cfg.Management.StoreIDs[*storeIndex]}
				fmt.Printf("从配置文件读取店铺ID[%d]: %d\n", *storeIndex, targetStoreIDs[0])
			} else if *storeIndex == 0 {
				// 默认使用第一个
				targetStoreIDs = []int64{cfg.Management.StoreIDs[0]}
				fmt.Printf("从配置文件读取店铺ID（默认第一个）: %d\n", targetStoreIDs[0])
			} else {
				fmt.Printf("错误: 店铺索引 %d 超出范围，配置文件中共有 %d 个店铺ID\n", *storeIndex, len(cfg.Management.StoreIDs))
				fmt.Println("使用 -list-stores 查看所有可用的店铺ID")
				os.Exit(1)
			}
		}

		if len(targetStoreIDs) == 0 {
			fmt.Println("错误: 必须指定店铺ID")
			fmt.Println("可用选项:")
			fmt.Println("  -store=ID          指定单个店铺ID")
			fmt.Println("  -store-ids=ID1,ID2 指定多个店铺ID")
			fmt.Println("  -all-stores        处理所有配置的店铺")
			fmt.Println("  -store-index=N     使用配置文件中第N个店铺")
			fmt.Println("使用 -list-stores 查看配置文件中的店铺ID")
			flag.Usage()
			os.Exit(1)
		}
	}

	fmt.Printf("=== TEMU 批量重新上架工具 ===\n")
	fmt.Printf("租户ID: %d\n", *tenantID)
	fmt.Printf("店铺数量: %d\n", len(targetStoreIDs))
	fmt.Printf("店铺ID列表: %v\n", targetStoreIDs)
	fmt.Printf("请求间隔: %d毫秒\n", *delay)
	fmt.Printf("并发数量: %d\n", *concurrency)
	fmt.Printf("处理模式: %s\n", func() string {
		if *firstPageOnly {
			return "循环处理第一页"
		}
		return "处理所有页面"
	}())
	if *dryRun {
		fmt.Printf("模式: 试运行（不会实际上架）\n")
	}
	fmt.Printf("================================\n\n")

	// 创建认证客户端
	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		cfg.Management.TenantID,
		logrus.StandardLogger(),
	)

	// 获取访问令牌
	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		fmt.Printf("错误: 获取访问令牌失败: %v\n", err)
		os.Exit(1)
	}

	// 创建管理客户端
	managementClient := management.NewClientManager(&cfg.Management)
	if managementClient == nil {
		fmt.Println("错误: 无法创建管理客户端")
		os.Exit(1)
	}

	// 设置访问令牌
	client := managementClient.GetClient()
	client.SetUserToken(accessToken, cfg.Management.TenantID)

	// 多店铺处理
	type StoreResult struct {
		StoreID int64                 `json:"store_id"`
		Result  *temu.RelistAllResult `json:"result"`
		Error   string                `json:"error,omitempty"`
	}

	var allResults []StoreResult
	totalSuccess := 0
	totalFail := 0
	totalSkipped := 0
	totalProcessed := 0

	for i, storeID := range targetStoreIDs {
		fmt.Printf("\n=== 处理店铺 %d (%d/%d) ===\n", storeID, i+1, len(targetStoreIDs))

		// 创建TEMU API客户端
		apiClient := temu.NewAPIClient(*tenantID, storeID, managementClient)
		if apiClient == nil {
			fmt.Printf("错误: 无法创建店铺 %d 的TEMU API客户端\n", storeID)
			allResults = append(allResults, StoreResult{
				StoreID: storeID,
				Error:   "无法创建TEMU API客户端",
			})
			continue
		}

		// 创建批量重新上架服务
		service := temu.NewBulkRelistService(apiClient)

		// 构建选项
		options := &temu.BulkRelistOptions{
			DelayBetweenRequests: *delay,
			MaxConcurrency:       *concurrency,
			ProcessFirstPageOnly: *firstPageOnly,
			SkipConditions: &temu.SkipConditions{
				SkipNeedRectification: *skipRectify,
				SkipSeverelyPunished:  *skipPunished,
				SkipLocked:            *skipLocked,
				SkipNoStock:           *skipNoStock,
				MinStock:              *minStock,
			},
			DryRun: *dryRun,
		}

		var result *temu.RelistAllResult
		var err error

		// 检查是否有过滤条件
		hasFilter := *includeCategories != "" || *excludeCategories != "" ||
			*nameKeywords != "" || *minPrice > 0 || *maxPrice > 0

		if hasFilter {
			// 使用过滤条件
			filter := &temu.ProductFilter{
				MinPrice: *minPrice,
				MaxPrice: *maxPrice,
			}

			if *includeCategories != "" {
				filter.IncludeCategories = strings.Split(*includeCategories, ",")
				for j := range filter.IncludeCategories {
					filter.IncludeCategories[j] = strings.TrimSpace(filter.IncludeCategories[j])
				}
			}

			if *excludeCategories != "" {
				filter.ExcludeCategories = strings.Split(*excludeCategories, ",")
				for j := range filter.ExcludeCategories {
					filter.ExcludeCategories[j] = strings.TrimSpace(filter.ExcludeCategories[j])
				}
			}

			if *nameKeywords != "" {
				filter.NameKeywords = strings.Split(*nameKeywords, ",")
				for j := range filter.NameKeywords {
					filter.NameKeywords[j] = strings.TrimSpace(filter.NameKeywords[j])
				}
			}

			fmt.Printf("店铺 %d: 使用过滤条件进行批量上架...\n", storeID)
			result, err = service.RelistOfflineProductsWithFilter(filter, options)
		} else {
			// 全部上架
			fmt.Printf("店铺 %d: 开始全部批量上架...\n", storeID)
			result, err = service.RelistAllOfflineProducts(options)
		}

		if err != nil {
			fmt.Printf("店铺 %d 处理失败: %v\n", storeID, err)
			allResults = append(allResults, StoreResult{
				StoreID: storeID,
				Error:   err.Error(),
			})
			continue
		}

		// 记录结果
		allResults = append(allResults, StoreResult{
			StoreID: storeID,
			Result:  result,
		})

		// 累计统计
		totalSuccess += result.SuccessCount
		totalFail += result.FailCount
		totalSkipped += result.SkippedCount
		totalProcessed += result.ProcessedCount

		// 显示店铺结果摘要
		fmt.Printf("店铺 %d 完成: 下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d\n",
			storeID, result.TotalOfflineCount, result.ProcessedCount,
			result.SuccessCount, result.FailCount, result.SkippedCount)

		// 店铺间添加延迟
		if i < len(targetStoreIDs)-1 {
			fmt.Printf("等待 %d 毫秒后处理下一个店铺...\n", *delay)
			time.Sleep(time.Duration(*delay) * time.Millisecond)
		}
	}

	// 显示总体结果摘要
	fmt.Printf("\n=== 多店铺批量上架完成 ===\n")
	fmt.Printf("处理店铺数: %d\n", len(targetStoreIDs))
	fmt.Printf("总处理商品数: %d\n", totalProcessed)
	fmt.Printf("总成功上架数: %d\n", totalSuccess)
	fmt.Printf("总失败数: %d\n", totalFail)
	fmt.Printf("总跳过数: %d\n", totalSkipped)

	if totalProcessed > 0 {
		successRate := float64(totalSuccess) / float64(totalProcessed) * 100
		fmt.Printf("总体成功率: %.2f%%\n", successRate)
	}

	// 显示各店铺详细结果
	fmt.Printf("\n=== 各店铺详细结果 ===\n")
	for _, storeResult := range allResults {
		if storeResult.Error != "" {
			fmt.Printf("店铺 %d: 处理失败 - %s\n", storeResult.StoreID, storeResult.Error)
		} else if storeResult.Result != nil {
			result := storeResult.Result
			fmt.Printf("店铺 %d: 下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
				storeResult.StoreID, result.TotalOfflineCount, result.ProcessedCount,
				result.SuccessCount, result.FailCount, result.SkippedCount)
			if result.ProcessedCount > 0 {
				rate := float64(result.SuccessCount) / float64(result.ProcessedCount) * 100
				fmt.Printf(" (成功率: %.1f%%)", rate)
			}
			fmt.Println()
		}
	}

	// 显示详细结果（仅在verbose模式下）
	if *verbose {
		fmt.Printf("\n=== 详细商品结果 ===\n")
		for _, storeResult := range allResults {
			if storeResult.Result != nil && len(storeResult.Result.Results) > 0 {
				fmt.Printf("\n店铺 %d 的商品详情:\n", storeResult.StoreID)
				for i, detail := range storeResult.Result.Results {
					status := "✓ 成功"
					if detail.Skipped {
						status = "⊘ 跳过"
					} else if !detail.Success {
						status = "✗ 失败"
					}

					fmt.Printf("  [%d] %s - %s (SKU数: %d)\n",
						i+1, status, detail.GoodsName, detail.SkuCount)

					if detail.Error != "" {
						fmt.Printf("      原因: %s\n", detail.Error)
					}
				}
			}
		}
	}

	// 输出到文件
	if *outputFile != "" {
		outputData := map[string]interface{}{
			"summary": map[string]interface{}{
				"total_stores":    len(targetStoreIDs),
				"total_processed": totalProcessed,
				"total_success":   totalSuccess,
				"total_fail":      totalFail,
				"total_skipped":   totalSkipped,
				"success_rate": func() float64 {
					if totalProcessed > 0 {
						return float64(totalSuccess) / float64(totalProcessed) * 100
					}
					return 0
				}(),
			},
			"store_results": allResults,
		}

		if err := saveResultToFile(outputData, *outputFile); err != nil {
			fmt.Printf("警告: 保存结果到文件失败: %v\n", err)
		} else {
			fmt.Printf("结果已保存到: %s\n", *outputFile)
		}
	}

	fmt.Printf("===================\n")
}

// saveResultToFile 保存结果到文件
func saveResultToFile(result interface{}, filename string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// 使用示例函数
func printUsageExamples() {
	fmt.Print(`
使用示例:

1. 查看配置文件中的店铺ID:
   ./temu-bulk-relist -list-stores

2. 基本用法 - 使用配置文件中的默认值:
   ./temu-bulk-relist

3. 处理所有配置的店铺:
   ./temu-bulk-relist -all-stores

4. 处理指定的多个店铺:
   ./temu-bulk-relist -store-ids="627,628,629"

5. 使用配置文件中的第二个店铺ID:
   ./temu-bulk-relist -store-index=1

6. 覆盖配置文件，使用指定的租户和单个店铺ID:
   ./temu-bulk-relist -tenant=123 -store=456

7. 自定义延迟、并发和跳过条件:
   ./temu-bulk-relist -delay=500 -concurrency=3 -skip-no-stock=true -min-stock=5

8. 高并发快速上架（谨慎使用）:
   ./temu-bulk-relist -concurrency=5 -delay=200

9. 按分类筛选:
   ./temu-bulk-relist -include-categories="电子产品,家居用品"

10. 按价格范围筛选:
    ./temu-bulk-relist -min-price=10.0 -max-price=100.0

11. 按商品名称关键词筛选:
    ./temu-bulk-relist -name-keywords="热销,新品"

12. 试运行模式（不实际上架）:
    ./temu-bulk-relist -dry-run=true

13. 详细输出并保存结果和日志:
    ./temu-bulk-relist -verbose=true -output=result.json -log-file=relist.log

14. 多店铺组合条件:
    ./temu-bulk-relist \
      -all-stores \
      -include-categories="电子产品" \
      -min-price=5.0 \
      -min-stock=10 \
      -delay=1500 \
      -log-file=multi_store_relist.log \
      -output=multi_store_result.json
`)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "TEMU 批量重新上架工具\n\n")
		fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		printUsageExamples()
	}
}
