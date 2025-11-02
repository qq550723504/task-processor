# TEMU Platform Processor

TEMU平台产品处理器，用于处理TEMU平台的产品发布流程。这是从原始TEMU项目迁移到task-processor架构的完整实现。

## 功能特性

### 核心功能
- ✅ 完整的产品发布管道（24个处理器）
- ✅ 支持Amazon产品数据处理和转换
- ✅ 动态管道构建（根据数据源自动调整）
- ✅ 图片处理和上传
- ✅ 分类推荐和验证
- ✅ SKU/SKC管理
- ✅ 价格处理（零售价、供应商价）
- ✅ 合规性检查
- ✅ 错误处理和重试机制

### 处理器列表
1. **InitDataHandler** - 初始化产品数据
2. **StoreInfoHandler** - 获取店铺信息
3. **RawJsonDataHandler** - 获取原始JSON数据
4. **TextCheckHandler** - 文本检查
5. **VariantJsonDataHandler** - 变体JSON数据处理
6. **CategoryRecommendHandler** - 分类推荐
7. **CategoryDisclaimHandler** - 分类免责声明
8. **CommitCreateHandler** - 提交创建
9. **CategoryHandler** - 分类处理
10. **ImageUploadHandler** - 图片上传
11. **ImageHandler** - 图片处理
12. **BuildSpuHandler** - 构建SPU
13. **ProductSubmitHandler** - 产品提交
14. **ProductSaveHandler** - 产品保存
15. **SkuCheckHandler** - SKU检查
16. **OutGoodsSnHandler** - 外部商品编号
17. **TemplateQueryHandler** - 模板查询
18. **MaxRetailPriceHandler** - 最大零售价格
19. **SupplierPriceHandler** - 供应商价格
20. **CompliancePhotoHandler** - 合规性照片
21. **ComplianceCertHandler** - 合规性认证
22. **CommitDetailHandler** - 提交详情
23. **CostTemplateHandler** - 成本模板
24. **PublishHandler** - 发布产品

### Amazon数据处理
- **AmazonDataHandler** - 专门处理Amazon产品数据
- 自动检测Amazon平台并插入Amazon数据处理器
- 支持Amazon产品信息转换为TEMU格式

## 项目结构

```
go/task-processor/platforms/temu/
├── handlers/                    # 处理器实现
│   ├── init_handler.go         # 初始化处理器
│   ├── store_info_handler.go   # 店铺信息处理器
│   ├── raw_json_data_handler.go # 原始数据处理器
│   ├── text_check_handler.go   # 文本检查处理器
│   ├── category_recommend_handler.go # 分类推荐处理器
│   ├── image_upload_handler.go # 图片上传处理器
│   ├── build_spu_handler.go    # SPU构建处理器
│   ├── product_submit_handler.go # 产品提交处理器
│   ├── publish_handler.go      # 发布处理器
│   └── remaining_handlers.go   # 其他处理器
├── types/                      # 类型定义
│   └── product.go             # TEMU产品结构体
├── example/                    # 示例代码
│   └── main.go                # 使用示例
├── processor.go               # 主处理器
├── pipeline.go               # 管道配置
├── task_handler.go           # 任务处理器
├── task_fetcher.go          # 任务获取器
├── errors.go                # 错误定义
└── README.md               # 说明文档
```

## 使用方法

### 1. 基本使用

```go
package main

import (
    "context"
    "task-processor/common/config"
    "task-processor/common/types"
    "task-processor/platforms/temu"
)

func main() {
    // 创建配置
    cfg := &config.Config{
        Amazon: config.AmazonConfig{
            Enabled:        true,
            PoolSize:       1,
            ViewportWidth:  1920,
            ViewportHeight: 1080,
            Headless:       true,
        },
        Processor: config.ProcessorConfig{
            MaxRetries: 3,
            Timeout:    300,
        },
    }

    // 创建TEMU处理器
    processor := temu.NewTemuProcessor(cfg)
    defer processor.Close()

    // 设置用户令牌
    processor.SetUserToken("your_access_token", "tenant_id")

    // 创建任务
    task := types.Task{
        ID:         "task_001",
        ProductID:  "B08N5WRWNW", // Amazon ASIN
        Platform:   "amazon",
        StoreID:    12345,
        TenantID:   "tenant_001",
        Priority:   100,
    }

    // 处理任务
    ctx := context.Background()
    err := processor.ProcessTask(ctx, task)
    if err != nil {
        log.Fatalf("处理失败: %v", err)
    }
}
```

### 2. 批量处理

```go
// 批量处理多个任务
tasks := []types.Task{
    {ID: "task_001", ProductID: "B08N5WRWNW", Platform: "amazon", StoreID: 12345},
    {ID: "task_002", ProductID: "B07XJ8C8F5", Platform: "amazon", StoreID: 12345},
    {ID: "task_003", ProductID: "TEMU_001", Platform: "temu", StoreID: 12345},
}

for _, task := range tasks {
    err := processor.ProcessTask(ctx, task)
    if err != nil {
        log.Printf("任务 %s 处理失败: %v", task.ID, err)
    }
}
```

## 配置说明

### Amazon配置
```yaml
amazon:
  enabled: true          # 是否启用Amazon爬虫
  pool_size: 1          # 浏览器池大小
  viewport_width: 1920  # 视口宽度
  viewport_height: 1080 # 视口高度
  headless: true        # 是否无头模式
```

### 处理器配置
```yaml
processor:
  max_retries: 3        # 最大重试次数
  timeout: 300          # 超时时间（秒）
```

## 数据流程

1. **任务接收** - 从Redis或其他队列接收任务
2. **数据获取** - 获取店铺信息和原始产品数据
3. **数据处理** - 根据平台类型处理数据（Amazon/TEMU）
4. **内容检查** - 文本检查、分类推荐等
5. **产品构建** - 构建TEMU产品结构
6. **图片处理** - 上传和处理产品图片
7. **价格设置** - 设置各种价格信息
8. **合规检查** - 进行合规性验证
9. **产品提交** - 提交到TEMU平台
10. **发布上架** - 发布产品到市场

## 错误处理

- 支持可重试错误自动重试
- 任务优先级动态调整
- 详细的错误日志记录
- 任务状态跟踪

## 扩展性

- 模块化设计，易于添加新的处理器
- 支持多平台数据源
- 可配置的处理管道
- 插件式架构

## 迁移说明

本项目是从原始TEMU项目（`go/temu/`）完整迁移而来，保持了以下兼容性：

1. **处理器逻辑** - 保持原有的业务逻辑不变
2. **数据结构** - 兼容原有的产品数据结构
3. **API接口** - 保持与TEMU平台的API兼容
4. **配置格式** - 支持原有的配置格式

### 主要改进

1. **架构优化** - 采用统一的task-processor架构
2. **错误处理** - 增强的错误处理和重试机制
3. **监控支持** - 更好的日志和监控支持
4. **扩展性** - 更好的模块化和扩展性
5. **性能优化** - 优化的并发处理能力

## 运行示例

```bash
cd go/task-processor/platforms/temu/example
go run main.go
```

## 注意事项

1. 确保配置了正确的TEMU API凭证
2. Amazon爬虫需要适当的网络环境
3. 图片上传需要足够的存储空间
4. 建议在生产环境中使用Redis作为任务队列