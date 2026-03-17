// package pipeline 提供Amazon上架流程测试
package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"task-processor/internal/amazon/model"

	"github.com/stretchr/testify/assert"
)

// TestAmazonUploadFlow 测试Amazon完整上架流程
func TestAmazonUploadFlow(t *testing.T) {
	t.Log("🚀 开始Amazon上架流程测试")

	// 创建服务容器
	services := model.NewServices()

	// 创建处理器管理器
	manager := NewHandlerManager(services)

	// 创建测试任务上下文
	taskContext := createTestTaskContext()

	// 执行完整处理流程
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	err := manager.ProcessProduct(ctx, taskContext)
	duration := time.Since(startTime)

	// 验证处理结果
	assert.NoError(t, err, "上架流程应该成功")
	assert.NotNil(t, taskContext.Results, "应该有处理结果")

	t.Logf("✅ 上架流程测试成功，耗时: %v", duration)

	// 验证关键处理结果
	validateResults(t, taskContext)

	// 打印摘要
	printSummary(t, taskContext, duration)
}

// createTestTaskContext 创建测试任务上下文
func createTestTaskContext() *model.TaskContext {
	return &model.TaskContext{
		TaskID:        "upload-flow-test-001",
		MarketplaceID: "ATVPDKIKX0DER",
		LanguageTag:   "en_US",
		Currency:      "USD",
		Data: map[string]any{
			"product_id": "test-upload-product-123",
			"store_id":   int64(1001),
			"tenant_id":  int64(1),
			"raw_json_data": `{
				"title": "韩版修身显瘦长袖连衣裙女装春秋新款",
				"brand": "时尚女装",
				"description": "优雅的韩版修身连衣裙，采用高品质面料，显瘦效果佳",
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
					}
				]
			}`,
		},
	}
}

// validateResults 验证处理结果
func validateResults(t *testing.T, taskContext *model.TaskContext) {
	// 验证数据解析结果
	rawProductData, exists := taskContext.GetResult("raw_product_data")
	assert.True(t, exists, "应该有解析后的产品数据")
	assert.NotNil(t, rawProductData, "解析后的产品数据不应为空")
	t.Log("✅ 数据解析: 成功")

	// 验证产品类型识别结果
	productType, exists := taskContext.GetResult("product_type")
	assert.True(t, exists, "应该有产品类型")
	assert.NotEmpty(t, productType, "产品类型不应为空")
	t.Logf("✅ 产品类型识别: %v", productType)

	// 验证属性映射结果
	mappedAttributes, exists := taskContext.GetResult("mapped_attributes")
	assert.True(t, exists, "应该有映射后的属性")
	assert.NotNil(t, mappedAttributes, "映射后的属性不应为空")
	t.Log("✅ 属性映射: 成功")

	// 验证Listing创建结果
	listingSKU, exists := taskContext.GetResult("listing_sku")
	assert.True(t, exists, "应该有Listing SKU")
	assert.NotEmpty(t, listingSKU, "Listing SKU不应为空")
	t.Logf("✅ Listing创建: SKU=%v", listingSKU)

	// 验证价格设置结果
	pricingAmount, exists := taskContext.GetResult("pricing_amount")
	assert.True(t, exists, "应该有价格信息")
	assert.NotNil(t, pricingAmount, "价格不应为空")
	t.Logf("✅ 价格设置: %v USD", pricingAmount)

	// 验证库存设置结果
	inventoryUpdated, exists := taskContext.GetResult("inventory_updated")
	assert.True(t, exists, "应该有库存更新状态")
	assert.Equal(t, true, inventoryUpdated, "库存应该已更新")
	t.Log("✅ 库存设置: 成功")
}

// printSummary 打印处理摘要
func printSummary(t *testing.T, taskContext *model.TaskContext, duration time.Duration) {
	separator := strings.Repeat("=", 50)

	t.Logf("\n%s", separator)
	t.Logf("📊 Amazon上架流程处理摘要")
	t.Logf("%s", separator)
	t.Logf("⏱️  总处理时间: %v", duration)
	t.Logf("🆔 任务ID: %s", taskContext.TaskID)
	t.Logf("🌍 目标市场: %s", taskContext.MarketplaceID)
	t.Logf("💱 目标货币: %s", taskContext.Currency)
	t.Logf("📋 处理结果数量: %d", len(taskContext.Results))

	// 列出所有处理结果
	t.Logf("🔑 处理结果:")
	for key := range taskContext.Results {
		t.Logf("   - %s", key)
	}

	t.Logf("%s", separator)
	t.Logf("✅ 上架流程测试完成 - 所有Handler正常工作")
	t.Logf("%s", separator)
}

// TestAmazonUploadFlow_Performance 测试上架流程性能
func TestAmazonUploadFlow_Performance(t *testing.T) {
	services := model.NewServices()
	manager := NewHandlerManager(services)

	productCount := 3
	totalDuration := time.Duration(0)

	t.Logf("🚀 开始性能测试，处理 %d 个产品", productCount)

	for i := 0; i < productCount; i++ {
		taskContext := createTestTaskContext()
		taskContext.TaskID = fmt.Sprintf("perf-test-%03d", i+1)
		taskContext.Data["product_id"] = fmt.Sprintf("perf-product-%03d", i+1)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		startTime := time.Now()
		err := manager.ProcessProduct(ctx, taskContext)
		duration := time.Since(startTime)

		cancel()

		assert.NoError(t, err, "产品 %d 处理应该成功", i+1)
		totalDuration += duration

		t.Logf("📦 产品 %d 处理完成，耗时: %v", i+1, duration)
	}

	avgDuration := totalDuration / time.Duration(productCount)

	t.Logf("\n📊 性能测试结果:")
	t.Logf("   产品数量: %d", productCount)
	t.Logf("   总耗时: %v", totalDuration)
	t.Logf("   平均耗时: %v", avgDuration)
	t.Logf("   处理速度: %.2f 产品/秒", float64(productCount)/totalDuration.Seconds())

	// 性能断言
	assert.Less(t, avgDuration, 5*time.Second, "平均处理时间应该少于5秒")
	t.Log("✅ 性能测试通过")
}

