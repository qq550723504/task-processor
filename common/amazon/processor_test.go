package amazon

import (
	"fmt"
	"os"
	"testing"
	"time"

	"task-processor/common/config"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAmazonProductFetch 测试Amazon产品抓取功能
func TestAmazonProductFetch(t *testing.T) {
	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 加载配置
	cfg := loadTestConfig(t)

	// 创建处理器
	processor := NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	// 测试用例
	testCases := []struct {
		name        string
		url         string
		zipcode     string
		expectError bool
		validate    func(t *testing.T, product *Product)
	}{
		{
			name:        "测试美国站点产品",
			url:         "https://www.amazon.com/dp/B0DGWLBV24",
			zipcode:     "10001",
			expectError: false,
			validate: func(t *testing.T, product *Product) {
				assert.NotEmpty(t, product.Title, "产品标题不应为空")
				assert.NotEmpty(t, product.Asin, "ASIN不应为空")
				assert.Equal(t, "B0DGWLBV24", product.Asin, "ASIN应该匹配")
				assert.Equal(t, "USD", product.Currency, "货币应该是USD")
				assert.NotZero(t, product.FinalPrice, "最终价格不应为0")
				logrus.Infof("✅ 产品标题: %s", product.Title)
				logrus.Infof("✅ 价格: %.2f %s", product.FinalPrice, product.Currency)
				logrus.Infof("✅ 评分: %.1f (%d 评论)", product.Rating, product.ReviewsCount)
			},
		},
		{
			name:        "测试日本站点产品",
			url:         "https://www.amazon.co.jp/dp/B0CX23V2ZK",
			zipcode:     "100-0001",
			expectError: false,
			validate: func(t *testing.T, product *Product) {
				assert.NotEmpty(t, product.Title, "产品标题不应为空")
				assert.Equal(t, "JPY", product.Currency, "货币应该是JPY")
				logrus.Infof("✅ 产品标题: %s", product.Title)
				logrus.Infof("✅ 价格: %.2f %s", product.FinalPrice, product.Currency)
			},
		},
		{
			name:        "测试阿联酋站点产品",
			url:         "https://www.amazon.ae/dp/B0FGXJ6R3Q",
			zipcode:     "",
			expectError: false,
			validate: func(t *testing.T, product *Product) {
				assert.NotEmpty(t, product.Title, "产品标题不应为空")
				assert.NotEmpty(t, product.Asin, "ASIN不应为空")
				assert.Equal(t, "B0FGXJ6R3Q", product.Asin, "ASIN应该匹配")
				assert.Equal(t, "AED", product.Currency, "货币应该是AED")
				assert.NotZero(t, product.FinalPrice, "最终价格不应为0")
				logrus.Infof("✅ 产品标题: %s", product.Title)
				logrus.Infof("✅ 价格: %.2f %s", product.FinalPrice, product.Currency)
				logrus.Infof("✅ 评分: %.1f (%d 评论)", product.Rating, product.ReviewsCount)
			},
		},
		{
			name:        "测试沙特站点产品",
			url:         "https://www.amazon.sa/dp/B0D7YYBTFN",
			zipcode:     "",
			expectError: false,
			validate: func(t *testing.T, product *Product) {
				assert.NotEmpty(t, product.Title, "产品标题不应为空")
				assert.NotEmpty(t, product.Asin, "ASIN不应为空")
				assert.Equal(t, "B0D7YYBTFN", product.Asin, "ASIN应该匹配")
				assert.Equal(t, "SAR", product.Currency, "货币应该是SAR")
				assert.NotZero(t, product.FinalPrice, "最终价格不应为0")
				logrus.Infof("✅ 产品标题: %s", product.Title)
				logrus.Infof("✅ 价格: %.2f %s", product.FinalPrice, product.Currency)
				logrus.Infof("✅ 评分: %.1f (%d 评论)", product.Rating, product.ReviewsCount)
			},
		},
		{
			name:        "测试不存在的产品",
			url:         "https://www.amazon.com/dp/INVALID123",
			zipcode:     "10001",
			expectError: true,
			validate: func(t *testing.T, product *Product) {
				// 应该返回错误
			},
		},
	}

	// 执行测试
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logrus.Infof("\n========== 开始测试: %s ==========", tc.name)
			logrus.Infof("URL: %s", tc.url)
			logrus.Infof("邮编: %s", tc.zipcode)

			startTime := time.Now()
			product, err := processor.Process(tc.url, tc.zipcode)
			duration := time.Since(startTime)

			logrus.Infof("处理耗时: %v", duration)

			if tc.expectError {
				assert.Error(t, err, "应该返回错误")
				logrus.Infof("✅ 预期错误: %v", err)
			} else {
				require.NoError(t, err, "不应该返回错误")
				require.NotNil(t, product, "产品不应为nil")

				// 执行自定义验证
				if tc.validate != nil {
					tc.validate(t, product)
				}

				// 通用验证
				assert.NotEmpty(t, product.URL, "URL不应为空")
				assert.NotEmpty(t, product.Zipcode, "邮编不应为空")
			}

			logrus.Infof("========== 测试完成: %s ==========\n", tc.name)
		})
	}
}

// TestAmazonBatchFetch 测试批量抓取功能
func TestAmazonBatchFetch(t *testing.T) {
	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)

	// 加载配置
	cfg := loadTestConfig(t)

	// 创建处理器
	processor := NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	// 批量请求
	requests := []ProductRequest{
		{
			URL:     "https://www.amazon.com/dp/B0DGWLBV24",
			Zipcode: "10001",
		},
		{
			URL:     "https://www.amazon.com/dp/B0CX23V2ZK",
			Zipcode: "10001",
		},
	}

	logrus.Infof("\n========== 开始批量测试 ==========")
	logrus.Infof("批量数量: %d", len(requests))

	startTime := time.Now()
	results := processor.ProcessBatch(requests)
	duration := time.Since(startTime)

	logrus.Infof("批量处理耗时: %v", duration)
	logrus.Infof("平均每个产品耗时: %v", duration/time.Duration(len(requests)))

	// 验证结果
	assert.Equal(t, len(requests), len(results), "结果数量应该匹配")

	successCount := 0
	for i, result := range results {
		logrus.Infof("\n--- 结果 %d ---", i+1)
		if result.Error != nil {
			logrus.Errorf("❌ 错误: %v", result.Error)
		} else {
			successCount++
			logrus.Infof("✅ 成功: %s", result.Product.Title)
			logrus.Infof("   价格: %.2f %s", result.Product.FinalPrice, result.Product.Currency)
		}
	}

	logrus.Infof("\n批量处理统计: 成功 %d/%d", successCount, len(requests))
	assert.Greater(t, successCount, 0, "至少应该有一个成功")

	logrus.Infof("========== 批量测试完成 ==========\n")
}

// TestAmazonProductDetails 测试产品详细信息提取
func TestAmazonProductDetails(t *testing.T) {
	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)

	// 加载配置
	cfg := loadTestConfig(t)

	// 创建处理器
	processor := NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	url := "https://www.amazon.com/dp/B0G2S2FLWH"
	zipcode := "94566"

	logrus.Infof("\n========== 测试产品详细信息提取 ==========")
	logrus.Infof("URL: %s", url)

	product, err := processor.Process(url, zipcode)
	require.NoError(t, err)
	require.NotNil(t, product)

	// 验证基本信息
	logrus.Infof("\n--- 基本信息 ---")
	logrus.Infof("标题: %s", product.Title)
	logrus.Infof("品牌: %s", product.Brand)
	logrus.Infof("ASIN: %s", product.Asin)
	logrus.Infof("Parent ASIN: %s", product.ParentAsin)

	assert.NotEmpty(t, product.Title, "标题不应为空")
	assert.NotEmpty(t, product.Asin, "ASIN不应为空")

	// 验证价格信息
	logrus.Infof("\n--- 价格信息 ---")
	logrus.Infof("初始价格: %.2f", product.InitialPrice)
	logrus.Infof("最终价格: %.2f", product.FinalPrice)
	logrus.Infof("货币: %s", product.Currency)
	logrus.Infof("可用性: %s", product.Availability)

	assert.NotZero(t, product.FinalPrice, "最终价格不应为0")
	assert.NotEmpty(t, product.Currency, "货币不应为空")

	// 验证评价信息
	logrus.Infof("\n--- 评价信息 ---")
	logrus.Infof("评分: %.1f", product.Rating)
	logrus.Infof("评论数: %d", product.ReviewsCount)

	// 验证图片
	logrus.Infof("\n--- 图片信息 ---")
	logrus.Infof("主图: %s", product.ImageURL)
	logrus.Infof("图片数量: %d", product.ImagesCount)
	logrus.Infof("图片列表: %d 张", len(product.Images))

	assert.NotEmpty(t, product.ImageURL, "主图不应为空")

	// 验证分类
	if len(product.Categories) > 0 {
		logrus.Infof("\n--- 分类信息 ---")
		for i, cat := range product.Categories {
			logrus.Infof("分类 %d: %s", i+1, cat)
		}
	}

	// 验证变体
	if len(product.Variations) > 0 {
		logrus.Infof("\n--- 变体信息 ---")
		logrus.Infof("变体数量: %d", len(product.Variations))
		for i, v := range product.Variations[:min(3, len(product.Variations))] {
			logrus.Infof("变体 %d: %s (ASIN: %s, 价格: %.2f)", i+1, v.Name, v.Asin, v.Price)
		}
	}

	// 验证产品详情
	if len(product.ProductDetails) > 0 {
		logrus.Infof("\n--- 产品详情 ---")
		for i, detail := range product.ProductDetails[:min(5, len(product.ProductDetails))] {
			logrus.Infof("%d. %s: %s", i+1, detail.Type, detail.Value)
		}
	}

	// 验证特性
	if len(product.Features) > 0 {
		logrus.Infof("\n--- 产品特性 ---")
		for i, feature := range product.Features[:min(3, len(product.Features))] {
			logrus.Infof("%d. %s", i+1, feature)
		}
	}

	logrus.Infof("\n========== 详细信息测试完成 ==========\n")
}

// TestAmazonZipcodeCache 测试邮编缓存功能
func TestAmazonZipcodeCache(t *testing.T) {
	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)

	// 加载配置
	cfg := loadTestConfig(t)
	cfg.PoolSize = 1 // 使用单个浏览器实例测试缓存

	// 创建处理器
	processor := NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	url := "https://www.amazon.com/dp/B0DGWLBV24"
	zipcode := "10001"

	logrus.Infof("\n========== 测试邮编缓存 ==========")

	// 第一次请求（设置邮编）
	logrus.Infof("第一次请求（设置邮编）...")
	start1 := time.Now()
	product1, err1 := processor.Process(url, zipcode)
	duration1 := time.Since(start1)

	require.NoError(t, err1)
	require.NotNil(t, product1)
	logrus.Infof("第一次请求耗时: %v", duration1)

	// 第二次请求（使用缓存的邮编）
	logrus.Infof("\n第二次请求（应该跳过邮编设置）...")
	start2 := time.Now()
	product2, err2 := processor.Process(url, zipcode)
	duration2 := time.Since(start2)

	require.NoError(t, err2)
	require.NotNil(t, product2)
	logrus.Infof("第二次请求耗时: %v", duration2)

	// 第二次应该更快（因为跳过了邮编设置）
	logrus.Infof("\n性能对比:")
	logrus.Infof("第一次: %v", duration1)
	logrus.Infof("第二次: %v", duration2)
	logrus.Infof("提升: %.1f%%", float64(duration1-duration2)/float64(duration1)*100)

	logrus.Infof("\n========== 邮编缓存测试完成 ==========\n")
}

// loadTestConfig 加载测试配置
func loadTestConfig(_ *testing.T) *config.AmazonConfig {
	// 优先从环境变量读取配置
	headless := os.Getenv("AMAZON_HEADLESS") != "false"
	browserPath := os.Getenv("AMAZON_BROWSER_PATH")
	if browserPath == "" {
		browserPath = "./chrome/chrome.exe"
	}

	poolSizeStr := os.Getenv("AMAZON_POOL_SIZE")
	poolSize := 1
	if poolSizeStr != "" {
		fmt.Sscanf(poolSizeStr, "%d", &poolSize)
	}

	cfg := &config.AmazonConfig{
		Enabled:        true,
		Headless:       headless,
		BrowserPath:    browserPath,
		PoolSize:       poolSize,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		ProxyServer:    os.Getenv("AMAZON_PROXY"),
	}

	logrus.Infof("测试配置:")
	logrus.Infof("  Headless: %v", cfg.Headless)
	logrus.Infof("  BrowserPath: %s", cfg.BrowserPath)
	logrus.Infof("  PoolSize: %d", cfg.PoolSize)
	logrus.Infof("  Proxy: %s", cfg.ProxyServer)

	return cfg
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
