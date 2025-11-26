package amazon

import (
	"os"
	"testing"
	"time"

	"task-processor/common/config"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVariationsDuplicateASINRealProduct 使用真实产品 B08937KYGJ 测试修复重复ASIN的问题
func TestVariationsDuplicateASINRealProduct(t *testing.T) {
	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 加载配置
	cfg := loadRealTestConfig(t)

	// 创建处理器
	processor := NewAmazonProcessor(cfg)
	defer processor.Shutdown()

	// 测试产品 B08937KYGJ - 这是一个有多个变体的产品
	url := "https://www.amazon.com/dp/B08937KYGJ"
	zipcode := "10001"

	logrus.Infof("\n========== 测试真实产品 B08937KYGJ 的变体数据 ==========")
	logrus.Infof("URL: %s", url)
	logrus.Infof("邮编: %s", zipcode)

	startTime := time.Now()
	product, err := processor.Process(url, zipcode)
	duration := time.Since(startTime)

	require.NoError(t, err, "处理产品时不应该返回错误")
	require.NotNil(t, product, "产品不应为nil")

	logrus.Infof("处理耗时: %v", duration)

	// 验证基本信息
	logrus.Infof("\n--- 基本信息 ---")
	logrus.Infof("标题: %s", product.Title)
	logrus.Infof("ASIN: %s", product.Asin)
	logrus.Infof("价格: %.2f %s", product.FinalPrice, product.Currency)

	assert.Equal(t, "B08937KYGJ", product.Asin, "ASIN应该匹配")
	assert.NotEmpty(t, product.Title, "标题不应为空")

	// 验证变体值（VariationsValues）
	logrus.Infof("\n--- 变体维度（VariationsValues）---")
	if len(product.VariationsValues) > 0 {
		for _, vv := range product.VariationsValues {
			logrus.Infof("维度: %s", vv.VariantName)
			logrus.Infof("  值: %v", vv.Values)
		}
	} else {
		logrus.Warnf("⚠️ 没有找到 VariationsValues 数据")
	}

	// 验证变体列表（Variations）
	logrus.Infof("\n--- 变体列表（Variations）---")
	logrus.Infof("变体总数: %d", len(product.Variations))

	if len(product.Variations) == 0 {
		t.Skip("产品没有变体数据，跳过变体测试")
		return
	}

	// 检查是否有重复的ASIN
	asinCount := make(map[string]int)
	asinToVariations := make(map[string][]string) // ASIN -> 变体名称列表

	for i, variation := range product.Variations {
		asinCount[variation.Asin]++
		asinToVariations[variation.Asin] = append(asinToVariations[variation.Asin], variation.Name)

		logrus.Infof("变体 %d:", i+1)
		logrus.Infof("  名称: %s", variation.Name)
		logrus.Infof("  ASIN: %s", variation.Asin)
		logrus.Infof("  价格: %.2f %s", variation.Price, variation.Currency)
		logrus.Infof("  属性: %+v", variation.Attributes)
	}

	// 验证没有重复的ASIN
	logrus.Infof("\n--- ASIN 唯一性检查 ---")
	duplicateFound := false
	for asin, count := range asinCount {
		if count > 1 {
			duplicateFound = true
			logrus.Errorf("❌ ASIN %s 出现了 %d 次（重复！）", asin, count)
			logrus.Errorf("   对应的变体: %v", asinToVariations[asin])
			t.Errorf("ASIN %s 出现了 %d 次，应该只出现1次", asin, count)
		} else {
			logrus.Infof("✅ ASIN %s 唯一", asin)
		}
	}

	if !duplicateFound {
		logrus.Infof("\n✅ 所有ASIN都是唯一的，没有重复！")
	}

	// 验证变体数量合理性
	assert.LessOrEqual(t, len(product.Variations), 100, "变体数量不应该超过100（可能是重复导致的）")

	// 验证每个变体都有必要的属性
	logrus.Infof("\n--- 变体属性完整性检查 ---")
	for i, variation := range product.Variations {
		assert.NotEmpty(t, variation.Asin, "变体 %d 的ASIN不应为空", i+1)
		assert.NotEmpty(t, variation.Name, "变体 %d 的名称不应为空", i+1)
		assert.NotNil(t, variation.Attributes, "变体 %d 的属性不应为nil", i+1)

		// 检查是否有 size 或 color 属性
		hasSize := variation.Attributes["size"] != nil
		hasColor := variation.Attributes["color"] != nil

		if !hasSize && !hasColor {
			logrus.Warnf("⚠️ 变体 %d (%s) 既没有 size 也没有 color 属性", i+1, variation.Name)
		}
	}

	// 特定验证：检查 "Black" 和 "Light Brown & Black" 是否被正确区分
	logrus.Infof("\n--- 颜色匹配精确性检查 ---")
	blackVariations := []string{}
	lightBrownBlackVariations := []string{}

	for _, variation := range product.Variations {
		if color, ok := variation.Attributes["color"].(string); ok {
			if color == "Black" {
				blackVariations = append(blackVariations, variation.Asin)
			} else if color == "Light Brown & Black" {
				lightBrownBlackVariations = append(lightBrownBlackVariations, variation.Asin)
			}
		}
	}

	logrus.Infof("纯 Black 颜色的变体: %v", blackVariations)
	logrus.Infof("Light Brown & Black 颜色的变体: %v", lightBrownBlackVariations)

	// 验证这两组ASIN没有交集
	for _, blackASIN := range blackVariations {
		for _, lbBlackASIN := range lightBrownBlackVariations {
			if blackASIN == lbBlackASIN {
				t.Errorf("❌ ASIN %s 同时出现在 Black 和 Light Brown & Black 中！", blackASIN)
			}
		}
	}

	if len(blackVariations) > 0 && len(lightBrownBlackVariations) > 0 {
		logrus.Infof("✅ Black 和 Light Brown & Black 颜色被正确区分")
	}

	// 统计信息
	logrus.Infof("\n--- 统计信息 ---")
	logrus.Infof("总变体数: %d", len(product.Variations))
	logrus.Infof("唯一ASIN数: %d", len(asinCount))
	logrus.Infof("重复ASIN数: %d", len(product.Variations)-len(asinCount))

	// 最终验证
	assert.Equal(t, len(asinCount), len(product.Variations),
		"唯一ASIN数应该等于变体总数（不应该有重复）")

	logrus.Infof("\n========== 测试完成 ==========\n")
}

// TestVariationsAttributeMatching 测试属性匹配的精确性
func TestVariationsAttributeMatching(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)

	// 创建提取器
	extractor := NewVariationsExtractor()

	// 测试用例：验证 "Black" 不会匹配到 "Light Brown & Black"
	testCases := []struct {
		name        string
		value1      string
		value2      string
		shouldMatch bool
	}{
		{
			name:        "精确匹配 - Black",
			value1:      "Black",
			value2:      "Black",
			shouldMatch: true,
		},
		{
			name:        "大小写不敏感 - Black vs black",
			value1:      "Black",
			value2:      "black",
			shouldMatch: true,
		},
		{
			name:        "不应该匹配 - Black vs Light Brown & Black",
			value1:      "Black",
			value2:      "Light Brown & Black",
			shouldMatch: false,
		},
		{
			name:        "不应该匹配 - Black vs Natural & Black",
			value1:      "Black",
			value2:      "Natural & Black",
			shouldMatch: false,
		},
		{
			name:        "精确匹配 - Light Brown & Black",
			value1:      "Light Brown & Black",
			value2:      "Light Brown & Black",
			shouldMatch: true,
		},
		{
			name:        "特殊字符处理 - L=17.4\" (1pcs)",
			value1:      "L=17.4\" (1pcs)",
			value2:      "L=17.4\"(1pcs)",
			shouldMatch: true,
		},
	}

	logrus.Infof("\n========== 测试属性匹配精确性 ==========")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractor.ValuesMatchGeneric(tc.value1, tc.value2)

			if tc.shouldMatch {
				assert.True(t, result, "%s 应该匹配 %s", tc.value1, tc.value2)
				logrus.Infof("✅ %s: '%s' == '%s' (匹配)", tc.name, tc.value1, tc.value2)
			} else {
				assert.False(t, result, "%s 不应该匹配 %s", tc.value1, tc.value2)
				logrus.Infof("✅ %s: '%s' != '%s' (不匹配)", tc.name, tc.value1, tc.value2)
			}
		})
	}

	logrus.Infof("\n========== 测试完成 ==========\n")
}

// loadRealTestConfig 加载真实测试配置
func loadRealTestConfig(t *testing.T) *config.AmazonConfig {
	// 优先从环境变量读取配置
	headless := os.Getenv("AMAZON_HEADLESS") != "false"
	browserPath := os.Getenv("AMAZON_BROWSER_PATH")
	if browserPath == "" {
		browserPath = "./chrome/chrome.exe"
	}

	cfg := &config.AmazonConfig{
		Enabled:        true,
		Headless:       headless,
		BrowserPath:    browserPath,
		PoolSize:       1,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		ProxyServer:    os.Getenv("AMAZON_PROXY"),
	}

	logrus.Infof("测试配置:")
	logrus.Infof("  Headless: %v", cfg.Headless)
	logrus.Infof("  BrowserPath: %s", cfg.BrowserPath)
	logrus.Infof("  Proxy: %s", cfg.ProxyServer)

	return cfg
}
