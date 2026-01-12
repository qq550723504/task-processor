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
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/services/product"
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
		concurrency       = flag.Int("concurrency", 25, "并发数量（1=串行，>1=并发）")
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
		firstPageOnly     = flag.Bool("first-page-only", false, "循环处理第一页模式（true=循环第一页，false=批量获取模式，推荐false）")
		processMode       = flag.String("mode", "batch", "处理模式：batch=批量获取模式（推荐），loop=循环第一页模式")
		showFailedStats   = flag.Bool("show-failed-stats", false, "显示失败商品统计信息并退出")
	)
	flag.Parse()

	// 设置日志级别
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// 设置日志输出到文件（必须在创建任何组件之前设置）
	var logFileHandle *os.File
	var logWriter io.Writer = os.Stdout // 默认只输出到控制台

	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			os.Exit(1)
		}
		logFileHandle = file

		// 同时输出到控制台和文件
		logWriter = io.MultiWriter(os.Stdout, file)
		logrus.SetOutput(logWriter)
	}

	// 确保在程序结束时关闭日志文件
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}

	// 创建自定义输出函数，确保所有输出都能写入日志文件
	logPrintf := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		fmt.Fprint(logWriter, message)
		// 强制刷新输出缓冲区，确保在调试时能立即看到输出
		if logFileHandle != nil {
			logFileHandle.Sync()
		}
	}

	// 添加启动日志，确保程序正常运行
	logrus.Info("程序启动，开始解析命令行参数")
	logPrintf("TEMU批量重新上架工具启动中...\n")

	// 加载配置
	logrus.Info("开始加载配置文件")
	cfg := config.LoadConfig()
	if cfg == nil {
		logrus.Error("无法加载配置文件")
		logPrintf("错误: 无法加载配置文件\n")
		os.Exit(1)
	}
	logrus.Info("配置文件加载完成")

	// 如果用户要求列出店铺ID，则显示并退出
	if *listStores {
		logrus.Info("用户请求列出店铺ID")
		logPrintf("配置文件中的店铺ID列表:\n")
		if len(cfg.Management.StoreIDs) == 0 {
			logPrintf("  (无店铺ID配置)\n")
		} else {
			for i, storeID := range cfg.Management.StoreIDs {
				logPrintf("  [%d] %d\n", i, storeID)
			}
		}
		logPrintf("租户ID: %s\n", cfg.Management.TenantID)
		logrus.Info("店铺ID列表显示完成，程序退出")
		os.Exit(0)
	}

	// 如果用户要求显示失败商品统计信息
	if *showFailedStats {
		logPrintf("失败商品统计功能已移除\n")
		os.Exit(0)
	}

	// 参数校验和配置处理
	if *tenantID == 0 {
		// 尝试从配置文件获取
		if cfg.Management.TenantID != "" {
			if tid, err := strconv.ParseInt(cfg.Management.TenantID, 10, 64); err == nil {
				*tenantID = tid
				logPrintf("从配置文件读取租户ID: %d\n", *tenantID)
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
		logPrintf("使用配置文件中的所有店铺: %v\n", targetStoreIDs)
	} else if *storeIDs != "" {
		// 解析命令行指定的多个店铺ID
		storeIDStrings := strings.Split(*storeIDs, ",")
		for _, idStr := range storeIDStrings {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				targetStoreIDs = append(targetStoreIDs, id)
			} else {
				logPrintf("错误: 无效的店铺ID '%s'\n", idStr)
				os.Exit(1)
			}
		}
		logPrintf("使用指定的店铺ID: %v\n", targetStoreIDs)
	} else if *storeID != 0 {
		// 使用单个店铺ID
		targetStoreIDs = []int64{*storeID}
		logPrintf("使用单个店铺ID: %d\n", *storeID)
	} else {
		// 根据storeIndex选择店铺ID
		if len(cfg.Management.StoreIDs) > 0 {
			if *storeIndex >= 0 && *storeIndex < len(cfg.Management.StoreIDs) {
				targetStoreIDs = []int64{cfg.Management.StoreIDs[*storeIndex]}
				logPrintf("从配置文件读取店铺ID[%d]: %d\n", *storeIndex, targetStoreIDs[0])
			} else if *storeIndex == 0 {
				// 默认使用第一个
				targetStoreIDs = []int64{cfg.Management.StoreIDs[0]}
				logPrintf("从配置文件读取店铺ID（默认第一个）: %d\n", targetStoreIDs[0])
			} else {
				logPrintf("错误: 店铺索引 %d 超出范围，配置文件中共有 %d 个店铺ID\n", *storeIndex, len(cfg.Management.StoreIDs))
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

	logPrintf("=== TEMU 批量重新上架工具 ===\n")
	logPrintf("租户ID: %d\n", *tenantID)
	logPrintf("店铺数量: %d\n", len(targetStoreIDs))
	logPrintf("店铺ID列表: %v\n", targetStoreIDs)
	logPrintf("请求间隔: %d毫秒\n", *delay)
	logPrintf("并发数量: %d\n", *concurrency)
	logPrintf("处理模式: %s\n", func() string {
		switch *processMode {
		case "loop":
			return "循环处理第一页（适合少量商品）"
		case "batch":
			return "批量获取模式（推荐，处理所有商品）"
		default:
			if *firstPageOnly {
				return "循环处理第一页（适合少量商品）"
			}
			return "批量获取模式（推荐，处理所有商品）"
		}
	}())
	if *dryRun {
		logPrintf("模式: 试运行（不会实际上架）\n")
	}
	logPrintf("================================\n\n")

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
		logPrintf("错误: 获取访问令牌失败: %v\n", err)
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
		StoreID int64                   `json:"store_id"`
		Result  *models.RelistAllResult `json:"result"`
		Error   string                  `json:"error,omitempty"`
	}

	var allResults []StoreResult
	totalSuccess := 0
	totalFail := 0
	totalSkipped := 0
	totalProcessed := 0

	for i, storeID := range targetStoreIDs {
		logPrintf("\n=== 处理店铺 %d (%d/%d) ===\n", storeID, i+1, len(targetStoreIDs))

		// 创建TEMU API客户端
		apiClient := api.NewAPIClient(*tenantID, storeID, managementClient)
		if apiClient == nil {
			logPrintf("错误: 无法创建店铺 %d 的TEMU API客户端\n", storeID)
			allResults = append(allResults, StoreResult{
				StoreID: storeID,
				Error:   "无法创建TEMU API客户端",
			})
			continue
		}

		// 创建批量重新上架服务
		service := product.NewBulkRelistService(apiClient)

		// 处理模式参数
		var useFirstPageOnly bool
		switch *processMode {
		case "loop":
			useFirstPageOnly = true
		case "batch":
			useFirstPageOnly = false
		default:
			// 如果模式参数无效，使用 first-page-only 参数
			useFirstPageOnly = *firstPageOnly
		}

		// 构建选项
		options := &models.BulkRelistOptions{
			DelayBetweenRequests: *delay,
			MaxConcurrency:       *concurrency,
			ProcessFirstPageOnly: useFirstPageOnly,
			SkipConditions: &models.SkipConditions{
				SkipNeedRectification: *skipRectify,
				SkipSeverelyPunished:  *skipPunished,
				SkipLocked:            *skipLocked,
				SkipNoStock:           *skipNoStock,
				MinStock:              *minStock,
			},
			DryRun: *dryRun,
		}

		var result *models.RelistAllResult
		var err error

		// 检查是否有过滤条件
		hasFilter := *includeCategories != "" || *excludeCategories != "" ||
			*nameKeywords != "" || *minPrice > 0 || *maxPrice > 0

		if hasFilter {
			// 使用过滤条件
			filter := &models.ProductFilter{
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

			logPrintf("店铺 %d: 使用过滤条件进行批量上架...\n", storeID)
			result, err = service.RelistOfflineProductsWithFilter(filter, options)
		} else {
			// 全部上架
			logPrintf("店铺 %d: 开始全部批量上架...\n", storeID)
			result, err = service.RelistAllOfflineProducts(options)
		}

		if err != nil {
			logPrintf("店铺 %d 处理失败: %v\n", storeID, err)
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
		logPrintf("店铺 %d 完成: 下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d\n",
			storeID, result.TotalOfflineCount, result.ProcessedCount,
			result.SuccessCount, result.FailCount, result.SkippedCount)

		// 店铺间添加延迟
		if i < len(targetStoreIDs)-1 {
			logPrintf("等待 %d 毫秒后处理下一个店铺...\n", *delay)
			time.Sleep(time.Duration(*delay) * time.Millisecond)
		}
	}

	// 显示总体结果摘要
	logPrintf("\n=== 多店铺批量上架完成 ===\n")
	logPrintf("处理店铺数: %d\n", len(targetStoreIDs))
	logPrintf("总处理商品数: %d\n", totalProcessed)
	logPrintf("总成功上架数: %d\n", totalSuccess)
	logPrintf("总失败数: %d\n", totalFail)
	logPrintf("总跳过数: %d\n", totalSkipped)

	if totalProcessed > 0 {
		successRate := float64(totalSuccess) / float64(totalProcessed) * 100
		logPrintf("总体成功率: %.2f%%\n", successRate)
	}

	// 显示各店铺详细结果
	logPrintf("\n=== 各店铺详细结果 ===\n")
	for _, storeResult := range allResults {
		if storeResult.Error != "" {
			logPrintf("店铺 %d: 处理失败 - %s\n", storeResult.StoreID, storeResult.Error)
		} else if storeResult.Result != nil {
			result := storeResult.Result
			logPrintf("店铺 %d: 下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
				storeResult.StoreID, result.TotalOfflineCount, result.ProcessedCount,
				result.SuccessCount, result.FailCount, result.SkippedCount)
			if result.ProcessedCount > 0 {
				rate := float64(result.SuccessCount) / float64(result.ProcessedCount) * 100
				logPrintf(" (成功率: %.1f%%)", rate)
			}
			fmt.Println()
		}
	}

	// 显示详细结果（仅在verbose模式下）
	if *verbose {
		logPrintf("\n=== 详细商品结果 ===\n")
		for _, storeResult := range allResults {
			if storeResult.Result != nil && len(storeResult.Result.Results) > 0 {
				logPrintf("\n店铺 %d 的商品详情:\n", storeResult.StoreID)
				for i, detail := range storeResult.Result.Results {
					status := "✓ 成功"
					if detail.Skipped {
						status = "⊘ 跳过"
					} else if !detail.Success {
						status = "✗ 失败"
					}

					logPrintf("  [%d] %s - %s (SKU数: %d)\n",
						i+1, status, detail.GoodsName, detail.SkuCount)

					if detail.Error != "" {
						logPrintf("      原因: %s\n", detail.Error)
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
			logPrintf("警告: 保存结果到文件失败: %v\n", err)
		} else {
			logPrintf("结果已保存到: %s\n", *outputFile)
		}
	}

	logPrintf("===================\n")
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

2. 查看失败商品统计信息:
   ./temu-bulk-relist -show-failed-stats

3. 基本用法 - 使用配置文件中的默认值:
   ./temu-bulk-relist

4. 处理所有配置的店铺:
   ./temu-bulk-relist -all-stores

5. 处理指定的多个店铺:
   ./temu-bulk-relist -store-ids="627,628,629"

6. 使用配置文件中的第二个店铺ID:
   ./temu-bulk-relist -store-index=1

7. 覆盖配置文件，使用指定的租户和单个店铺ID:
   ./temu-bulk-relist -tenant=123 -store=456

8. 自定义延迟、并发和跳过条件:
   ./temu-bulk-relist -delay=500 -concurrency=3 -skip-no-stock=true -min-stock=5

9. 高并发快速上架（谨慎使用）:
   ./temu-bulk-relist -concurrency=5 -delay=200

10. 按分类筛选:
    ./temu-bulk-relist -include-categories="电子产品,家居用品"

11. 按价格范围筛选:
    ./temu-bulk-relist -min-price=10.0 -max-price=100.0

12. 按商品名称关键词筛选:
    ./temu-bulk-relist -name-keywords="热销,新品"

13. 试运行模式（不实际上架）:
    ./temu-bulk-relist -dry-run=true

14. 详细输出并保存结果和日志:
    ./temu-bulk-relist -verbose=true -output=result.json -log-file=relist.log

15. 清理超过7天的失败记录:
    ./temu-bulk-relist -clean-failed-days=7

16. 重置失败商品列表（清空所有失败记录）:
    ./temu-bulk-relist -reset-failed-list

18. 打印详细商品数据进行分析:
    ./temu-bulk-relist -store=508 -print-product-data=true -verbose=true

19. 多店铺组合条件:
    ./temu-bulk-relist \
      -all-stores \
      -include-categories="电子产品" \
      -min-price=5.0 \
      -min-stock=10 \
      -delay=1500 \
      -clean-failed-days=30 \
      -print-product-data=true \
      -log-file=multi_store_relist.log \
      -output=multi_store_result.json

失败商品管理说明:
- 系统会自动记录上架失败的商品ID到文件中
- 下次运行时会自动跳过这些失败的商品，避免重复尝试
- 使用 -show-failed-stats 查看当前失败商品统计
- 使用 -clean-failed-days=N 清理N天前的失败记录
- 使用 -reset-failed-list 清空所有失败记录
- 使用 -print-product-data=true 打印成功和失败商品的详细原始数据，用于分析哪些字段导致上架失败
- 失败记录文件位置: data/failed_goods/failed_goods_tenant_X_store_Y.json
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
