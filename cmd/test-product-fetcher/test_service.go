// Package main 提供测试服务实现
package main

import (
	"context"
	"fmt"

	"task-processor/internal/common/product"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

// TestService 测试服务
type TestService struct {
	ctx         context.Context
	cfg         *config.Config
	logger      *logrus.Entry
	fetcher     *product.ProductFetcher
	initializer *TestInitializer
}

// NewTestService 创建测试服务
func NewTestService(ctx context.Context, cfg *config.Config, logger *logrus.Entry) (*TestService, error) {
	// 创建初始化器
	initializer := NewTestInitializer(ctx, cfg, logger)

	// 初始化依赖
	if err := initializer.Initialize(); err != nil {
		return nil, fmt.Errorf("初始化依赖失败: %w", err)
	}

	// 创建ProductFetcher
	fetcher := product.NewProductFetcher(
		initializer.GetManagementClient().GetRawJsonDataClient(),
		&cfg.Amazon,
		initializer.GetAmazonProcessor(),
	)

	return &TestService{
		ctx:         ctx,
		cfg:         cfg,
		logger:      logger,
		fetcher:     fetcher,
		initializer: initializer,
	}, nil
}

// Cleanup 清理资源
func (ts *TestService) Cleanup() {
	if ts.initializer != nil {
		ts.initializer.Cleanup()
	}
}

// RunBasicTest 运行基础测试
func (ts *TestService) RunBasicTest() {
	fmt.Println("🧪 开始运行FetchProduct基础测试...")

	// 测试用例1：正常获取产品
	req := &product.FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B08BKKQ2NL", // Echo Dot 4th Gen
		StoreID:   617,
		Creator:   "test_user",
	}

	fmt.Printf("📋 测试请求: %+v\n", req)

	productResult, err := ts.fetcher.FetchProduct(req)
	if err != nil {
		fmt.Printf("❌ 测试失败: %v\n", err)
		return
	}

	if productResult != nil {
		fmt.Printf("✅ 测试成功: 获取到产品 %s - %s\n", productResult.Asin, productResult.Title)
		fmt.Printf("   价格: %.2f %s\n", productResult.FinalPrice, productResult.Currency)
		fmt.Printf("   评分: %.1f (%d 评价)\n", productResult.Rating, productResult.ReviewsCount)
	} else {
		fmt.Println("⚠️ 测试结果: 产品为空")
	}
}
