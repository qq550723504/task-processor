# Amazon 平台集成指南

## 概述

本指南将帮助你将 Amazon 平台上架功能集成到现有的任务处理系统中。

## 前置条件

1. ✅ 已有 SHEIN 或 TEMU 平台的实现
2. ✅ 已配置管理系统 API
3. ✅ 已获取 Amazon SP-API 凭证
4. ✅ Go 1.19+ 环境

## 集成步骤

### 第一步：配置 Amazon 凭证

在 `config/config-dev.yaml` 中添加 Amazon 配置：

```yaml
# Amazon配置
amazon:
  # Amazon 爬虫配置（用于抓取产品数据）
  enabled: true
  headless: true
  browserPath: "./chrome/chrome.exe"
  poolSize: 3
  viewportWidth: 1920
  viewportHeight: 1080
  dataFreshnessDays: 15
  
  # Amazon SP-API 配置（用于上架）
  spapi:
    enabled: true
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    clientID: "amzn1.application-oa2-client.xxxxx"
    clientSecret: "your-client-secret"
    refreshToken: "Atzr|your-refresh-token"
    defaultFulfillmentType: "FBM"
    defaultCondition: "New"

# 自动定价配置
autoPricing:
  amazon:
    enabled: false
    interval: 300
    batchSize: 100
```

### 第二步：在主程序中集成 Amazon 处理器

#### 方式1：独立运行（推荐用于测试）

创建 `cmd/amazon-listing/main.go`：

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "task-processor/common/config"
    "task-processor/platforms/amazon"
    
    "github.com/sirupsen/logrus"
)

func main() {
    // 加载配置
    cfg := config.LoadConfig("config/config-dev.yaml")
    
    // 创建日志
    logger := logrus.New()
    logger.SetLevel(logrus.InfoLevel)
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    
    // 创建 Amazon 处理器
    processor := amazon.NewAmazonProcessor(cfg, logger)
    
    // 启动处理器
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    if err := processor.Start(ctx); err != nil {
        logger.Fatalf("启动 Amazon 处理器失败: %v", err)
    }
    
    logger.Info("Amazon 处理器已启动，等待任务...")
    
    // 等待退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    logger.Info("收到退出信号，正在关闭...")
    processor.Close()
    logger.Info("Amazon 处理器已关闭")
}
```

运行：
```bash
go run cmd/amazon-listing/main.go
```

#### 方式2：与现有平台统一管理

修改现有的 `cmd/task-processor/main.go`，添加 Amazon 处理器：

```go
package main

import (
    "context"
    "task-processor/common/config"
    "task-processor/platforms/amazon"
    "task-processor/platforms/shein"
    "task-processor/platforms/temu"
    
    "github.com/sirupsen/logrus"
)

func main() {
    cfg := config.LoadConfig("config/config-dev.yaml")
    logger := logrus.New()
    ctx := context.Background()
    
    // 创建共享的管理客户端
    managementClient := management.NewClientManager(&cfg.Management)
    
    // 创建各平台处理器
    var processors []Processor
    
    // SHEIN 处理器
    sheinProcessor := shein.NewSheinProcessorWithManagementClient(
        cfg, managementClient,
    )
    processors = append(processors, sheinProcessor)
    
    // TEMU 处理器
    temuProcessor := temu.NewTemuProcessorWithManagementClient(
        cfg, logger, managementClient,
    )
    processors = append(processors, temuProcessor)
    
    // Amazon 处理器
    amazonProcessor := amazon.NewAmazonProcessorWithManagementClient(
        cfg, logger, managementClient,
    )
    processors = append(processors, amazonProcessor)
    
    // 启动所有处理器
    for _, p := range processors {
        if err := p.Start(ctx); err != nil {
            logger.Fatalf("启动处理器失败: %v", err)
        }
    }
    
    logger.Info("所有平台处理器已启动")
    
    // 等待退出信号...
}
```

### 第三步：创建统一的任务分发器

创建 `platforms/common/dispatcher.go`：

```go
package common

import (
    "context"
    "fmt"
    "task-processor/common/types"
    "task-processor/platforms/amazon"
    "task-processor/platforms/shein"
    "task-processor/platforms/temu"
    
    "github.com/sirupsen/logrus"
)

// TaskDispatcher 任务分发器
type TaskDispatcher struct {
    sheinProcessor  *shein.SheinProcessor
    temuProcessor   *temu.TemuProcessor
    amazonProcessor *amazon.AmazonProcessor
    logger          *logrus.Logger
}

// NewTaskDispatcher 创建任务分发器
func NewTaskDispatcher(
    sheinProc *shein.SheinProcessor,
    temuProc *temu.TemuProcessor,
    amazonProc *amazon.AmazonProcessor,
    logger *logrus.Logger,
) *TaskDispatcher {
    return &TaskDispatcher{
        sheinProcessor:  sheinProc,
        temuProcessor:   temuProc,
        amazonProcessor: amazonProc,
        logger:          logger,
    }
}

// DispatchTask 根据平台类型分发任务
func (d *TaskDispatcher) DispatchTask(ctx context.Context, task types.Task) error {
    // 根据任务的平台类型分发
    platform := task.Platform // 假设任务有 Platform 字段
    
    d.logger.WithFields(logrus.Fields{
        "taskID":   task.ID,
        "platform": platform,
    }).Info("分发任务")
    
    switch platform {
    case "shein":
        return d.sheinProcessor.ProcessTask(ctx, task)
    case "temu":
        return d.temuProcessor.ProcessTask(ctx, task)
    case "amazon":
        return d.amazonProcessor.ProcessTask(ctx, task)
    default:
        return fmt.Errorf("未知的平台类型: %s", platform)
    }
}
```

### 第四步：实现统一的任务获取器

创建 `platforms/common/fetcher.go`：

```go
package common

import (
    "context"
    "task-processor/common/management"
    "task-processor/common/types"
    "time"
    
    "github.com/sirupsen/logrus"
)

// UnifiedTaskFetcher 统一任务获取器
type UnifiedTaskFetcher struct {
    managementClient *management.ClientManager
    dispatcher       *TaskDispatcher
    logger           *logrus.Logger
    interval         time.Duration
}

// NewUnifiedTaskFetcher 创建统一任务获取器
func NewUnifiedTaskFetcher(
    mgmtClient *management.ClientManager,
    dispatcher *TaskDispatcher,
    logger *logrus.Logger,
    interval time.Duration,
) *UnifiedTaskFetcher {
    return &UnifiedTaskFetcher{
        managementClient: mgmtClient,
        dispatcher:       dispatcher,
        logger:           logger,
        interval:         interval,
    }
}

// Start 启动任务获取
func (f *UnifiedTaskFetcher) Start(ctx context.Context) {
    ticker := time.NewTicker(f.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            f.logger.Info("任务获取器停止")
            return
        case <-ticker.C:
            f.fetchAndDispatchTasks(ctx)
        }
    }
}

// fetchAndDispatchTasks 获取并分发任务
func (f *UnifiedTaskFetcher) fetchAndDispatchTasks(ctx context.Context) {
    // 从管理系统获取待处理任务
    tasks, err := f.fetchTasks(ctx)
    if err != nil {
        f.logger.Errorf("获取任务失败: %v", err)
        return
    }
    
    if len(tasks) == 0 {
        f.logger.Debug("没有待处理任务")
        return
    }
    
    f.logger.Infof("获取到 %d 个待处理任务", len(tasks))
    
    // 分发任务到对应的平台处理器
    for _, task := range tasks {
        go func(t types.Task) {
            if err := f.dispatcher.DispatchTask(ctx, t); err != nil {
                f.logger.Errorf("任务处理失败: TaskID=%s, Error=%v", t.ID, err)
            }
        }(task)
    }
}

// fetchTasks 从管理系统获取任务
func (f *UnifiedTaskFetcher) fetchTasks(ctx context.Context) ([]types.Task, error) {
    // TODO: 实现实际的任务获取逻辑
    // 调用管理系统 API 获取待处理任务
    return nil, nil
}
```

### 第五步：完整的主程序示例

创建 `cmd/unified-processor/main.go`：

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "task-processor/common/config"
    "task-processor/common/management"
    "task-processor/platforms/amazon"
    "task-processor/platforms/common"
    "task-processor/platforms/shein"
    "task-processor/platforms/temu"
    
    "github.com/sirupsen/logrus"
)

func main() {
    // 1. 加载配置
    cfg := config.LoadConfig("config/config-dev.yaml")
    
    // 2. 创建日志
    logger := logrus.New()
    logger.SetLevel(logrus.InfoLevel)
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    
    logger.Info("启动统一任务处理器...")
    
    // 3. 创建共享的管理客户端
    managementClient := management.NewClientManager(&cfg.Management)
    managementClient.SetDataFreshnessDays(cfg.Amazon.DataFreshnessDays)
    
    // 4. 创建各平台处理器
    logger.Info("初始化平台处理器...")
    
    sheinProcessor := shein.NewSheinProcessorWithManagementClient(
        cfg, managementClient,
    )
    
    temuProcessor := temu.NewTemuProcessorWithManagementClient(
        cfg, logger, managementClient,
    )
    
    amazonProcessor := amazon.NewAmazonProcessorWithManagementClient(
        cfg, logger, managementClient,
    )
    
    // 5. 启动所有处理器
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    logger.Info("启动 SHEIN 处理器...")
    if err := sheinProcessor.Start(ctx); err != nil {
        logger.Fatalf("启动 SHEIN 处理器失败: %v", err)
    }
    
    logger.Info("启动 TEMU 处理器...")
    if err := temuProcessor.Start(ctx); err != nil {
        logger.Fatalf("启动 TEMU 处理器失败: %v", err)
    }
    
    logger.Info("启动 Amazon 处理器...")
    if err := amazonProcessor.Start(ctx); err != nil {
        logger.Fatalf("启动 Amazon 处理器失败: %v", err)
    }
    
    // 6. 创建任务分发器
    dispatcher := common.NewTaskDispatcher(
        sheinProcessor,
        temuProcessor,
        amazonProcessor,
        logger,
    )
    
    // 7. 创建统一任务获取器
    fetcher := common.NewUnifiedTaskFetcher(
        managementClient,
        dispatcher,
        logger,
        30*time.Second, // 每30秒获取一次任务
    )
    
    // 8. 启动任务获取器
    go fetcher.Start(ctx)
    
    logger.Info("✅ 所有平台处理器已启动，开始处理任务...")
    
    // 9. 等待退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    logger.Info("收到退出信号，正在关闭...")
    
    // 10. 优雅关闭
    cancel()
    
    logger.Info("关闭 Amazon 处理器...")
    amazonProcessor.Close()
    
    logger.Info("关闭 TEMU 处理器...")
    temuProcessor.Close()
    
    logger.Info("关闭 SHEIN 处理器...")
    sheinProcessor.Close()
    
    logger.Info("✅ 所有处理器已关闭")
}
```

## 测试集成

### 单元测试

```bash
# 测试 Amazon 处理器
go test ./platforms/amazon/... -v

# 测试所有平台
go test ./platforms/... -v
```

### 集成测试

创建 `platforms/amazon/integration_test.go`：

```go
package amazon_test

import (
    "context"
    "testing"
    "time"
    
    "task-processor/common/config"
    "task-processor/common/types"
    "task-processor/platforms/amazon"
    
    "github.com/sirupsen/logrus"
)

func TestAmazonIntegration(t *testing.T) {
    // 加载配置
    cfg := &config.Config{
        Processor: config.ProcessorConfig{
            MaxRetries: 3,
        },
        Worker: config.WorkerConfig{
            Concurrency: 1,
        },
    }
    
    // 创建处理器
    logger := logrus.New()
    processor := amazon.NewAmazonProcessor(cfg, logger)
    
    // 启动
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := processor.Start(ctx); err != nil {
        t.Fatalf("启动失败: %v", err)
    }
    defer processor.Close()
    
    // 创建测试任务
    task := types.Task{
        ID:        "test-001",
        ProductID: "B08N5WRWNW",
        StoreID:   556,
        TenantID:  1,
    }
    
    // 处理任务
    err := processor.ProcessTask(ctx, task)
    if err != nil {
        t.Logf("任务处理失败（预期，因为没有实际配置）: %v", err)
    }
}
```

## 监控和日志

### 日志配置

```go
// 生产环境日志配置
logger := logrus.New()
logger.SetLevel(logrus.InfoLevel)
logger.SetFormatter(&logrus.JSONFormatter{})

// 输出到文件
file, err := os.OpenFile("logs/amazon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
if err == nil {
    logger.SetOutput(file)
}
```

### 监控指标

建议监控以下指标：

1. **任务处理指标**
   - 任务处理成功率
   - 任务处理耗时
   - 任务重试次数

2. **API 调用指标**
   - API 调用成功率
   - API 调用延迟
   - API 限流次数

3. **系统指标**
   - Worker 池使用率
   - 内存使用情况
   - Goroutine 数量

## 故障排查

### 常见问题

**Q1: Amazon 处理器无法启动**

检查配置文件是否正确：
```bash
cat config/config-dev.yaml | grep -A 10 "amazon:"
```

**Q2: 任务一直处于待处理状态**

检查 Worker 池状态：
```go
logger.Infof("可用 Worker: %d", processor.GetWorkerPool().AvailableSlots())
```

**Q3: API 调用失败**

检查 SP-API 凭证是否有效：
```bash
# 测试令牌刷新
curl -X POST https://api.amazon.com/auth/o2/token \
  -d "grant_type=refresh_token&refresh_token=YOUR_TOKEN&client_id=YOUR_ID&client_secret=YOUR_SECRET"
```

## 性能优化

### 并发配置

```yaml
worker:
  concurrency: 5  # 增加并发数
  bufferSize: 20  # 增加队列大小
```

### 批量处理

```go
// 批量更新库存
items := map[string]int{
    "SKU-001": 100,
    "SKU-002": 200,
    "SKU-003": 300,
}

err := inventoryService.BatchUpdateInventory(ctx, items)
```

## 下一步

1. ✅ 完成基础集成
2. ⏳ 实现 Amazon SP-API 实际调用
3. ⏳ 添加产品监控功能
4. ⏳ 实现自动定价策略
5. ⏳ 完善错误处理和重试

## 参考资料

- [Amazon SP-API 文档](https://developer-docs.amazon.com/sp-api/)
- [项目架构文档](./ARCHITECTURE.md)
- [快速开始指南](./QUICK_START.md)
