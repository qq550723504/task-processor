package shein

import (
	"context"
	"fmt"
	"task-processor/common/amazon"
	"task-processor/common/config"
	"task-processor/common/processor"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// SheinProcessor SHEIN平台处理器
type SheinProcessor struct {
	*processor.BaseProcessor
	config          *config.Config
	amazonProcessor *amazon.AmazonProcessor
}

// NewSheinProcessor 创建SHEIN处理器
func NewSheinProcessor(cfg *config.Config) *SheinProcessor {
	baseProcessor := processor.NewBaseProcessor(cfg)

	// 初始化Amazon处理器（如果启用）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled {
		amazonProcessor = amazon.NewAmazonProcessor(&cfg.Amazon)
		logrus.Info("[SHEIN] Amazon爬虫已启用")
	}

	return &SheinProcessor{
		BaseProcessor:   baseProcessor,
		config:          cfg,
		amazonProcessor: amazonProcessor,
	}
}

// ProcessTask 处理SHEIN任务
func (p *SheinProcessor) ProcessTask(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 开始处理任务: ID=%s, ProductID=%s, StoreID=%d",
		task.ID, task.ProductID, task.StoreID)

	// SHEIN特定的任务处理流程
	if err := p.processSheinProduct(ctx, task); err != nil {
		logrus.Errorf("[SHEIN] 处理产品失败: %v", err)
		return err
	}

	logrus.Infof("[SHEIN] 任务处理完成: ID=%s", task.ID)
	return nil
}

// processSheinProduct 处理SHEIN产品
func (p *SheinProcessor) processSheinProduct(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 处理产品: %s", task.ProductID)

	// 1. 初始化产品数据
	if err := p.initProductData(ctx, task); err != nil {
		return err
	}

	// 2. 获取原始JSON数据
	if err := p.getRawJsonData(ctx, task); err != nil {
		return err
	}

	// 3. 如果需要Amazon数据，使用Amazon爬虫
	if p.needsAmazonData(task) && p.amazonProcessor != nil {
		if err := p.fetchAmazonData(ctx, task); err != nil {
			logrus.Warnf("[SHEIN] Amazon数据获取失败: %v", err)
			// 不阻塞主流程，继续处理
		}
	}

	// 4. 处理变体JSON数据
	if err := p.processVariantJsonData(ctx, task); err != nil {
		return err
	}

	// 5. 构建属性
	if err := p.buildAttributes(ctx, task); err != nil {
		return err
	}

	// 6. 构建SKC列表
	if err := p.buildSkcList(ctx, task); err != nil {
		return err
	}

	// 7. 构建SPU
	if err := p.buildSpu(ctx, task); err != nil {
		return err
	}

	// 8. 发布产品
	if err := p.publishProduct(ctx, task); err != nil {
		return err
	}

	// 9. 保存发布结果
	if err := p.savePublishResult(ctx, task); err != nil {
		return err
	}

	return nil
}

// initProductData 初始化产品数据
func (p *SheinProcessor) initProductData(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 初始化产品数据: %s", task.ProductID)
	// 模拟初始化产品数据
	time.Sleep(100 * time.Millisecond)
	return nil
}

// getRawJsonData 获取原始JSON数据
func (p *SheinProcessor) getRawJsonData(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 获取原始JSON数据: %s", task.ProductID)
	// 模拟获取原始JSON数据
	time.Sleep(200 * time.Millisecond)
	return nil
}

// needsAmazonData 判断是否需要Amazon数据
func (p *SheinProcessor) needsAmazonData(task types.Task) bool {
	// 如果平台是Amazon或者产品ID包含Amazon相关信息
	return task.Platform == "amazon" || task.Platform == "Amazon"
}

// fetchAmazonData 使用Amazon爬虫获取数据
func (p *SheinProcessor) fetchAmazonData(ctx context.Context, task types.Task) error {
	if p.amazonProcessor == nil {
		return fmt.Errorf("Amazon爬虫未初始化")
	}

	// 构建Amazon URL
	amazonURL := fmt.Sprintf("https://www.amazon.com/dp/%s", task.ProductID)
	zipcode := "10001" // 可以从配置中获取

	logrus.Infof("[SHEIN] 使用Amazon爬虫获取数据: %s", amazonURL)

	product, err := p.amazonProcessor.Process(amazonURL, zipcode)
	if err != nil {
		return fmt.Errorf("Amazon爬虫处理失败: %w", err)
	}

	logrus.Infof("[SHEIN] Amazon数据获取完成: 标题=%s, 价格=%.2f %s",
		product.Title, product.FinalPrice, product.Currency)

	return nil
}

// processVariantJsonData 处理变体JSON数据
func (p *SheinProcessor) processVariantJsonData(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 处理变体JSON数据: %s", task.ProductID)
	// 模拟处理变体JSON数据
	time.Sleep(300 * time.Millisecond)
	return nil
}

// buildAttributes 构建属性
func (p *SheinProcessor) buildAttributes(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 构建属性: %s", task.ProductID)
	// 模拟构建属性
	time.Sleep(200 * time.Millisecond)
	return nil
}

// buildSkcList 构建SKC列表
func (p *SheinProcessor) buildSkcList(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 构建SKC列表: %s", task.ProductID)
	// 模拟构建SKC列表
	time.Sleep(250 * time.Millisecond)
	return nil
}

// buildSpu 构建SPU
func (p *SheinProcessor) buildSpu(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 构建SPU: %s", task.ProductID)
	// 模拟构建SPU
	time.Sleep(300 * time.Millisecond)
	return nil
}

// publishProduct 发布产品
func (p *SheinProcessor) publishProduct(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 发布产品: %s", task.ProductID)
	// 模拟发布产品
	time.Sleep(500 * time.Millisecond)
	return nil
}

// savePublishResult 保存发布结果
func (p *SheinProcessor) savePublishResult(ctx context.Context, task types.Task) error {
	logrus.Infof("[SHEIN] 保存发布结果: %s", task.ProductID)
	// 模拟保存发布结果
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Close 关闭处理器
func (p *SheinProcessor) Close() {
	logrus.Info("[SHEIN] 关闭SHEIN任务处理器")

	// 关闭Amazon处理器
	if p.amazonProcessor != nil {
		p.amazonProcessor.Shutdown()
	}

	// 调用基础处理器的关闭方法
	p.BaseProcessor.Close()
}
