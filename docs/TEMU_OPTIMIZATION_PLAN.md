# TEMU 平台代码优化方案

## 📋 优化目标

基于 Go 代码最佳实践规则，对 TEMU 平台代码进行优化，提升代码质量、可维护性和性能。

---

## 🎯 优化项清单

### P0 - 必须修复

#### 1. 拆分 pipeline.go 中的 addHandlers 方法
**问题**：单个方法包含 32 个处理器链式调用，代码过长且难以维护

**方案**：
```go
// 按业务阶段拆分为多个方法
func (b *TemuPipelineBuilder) addHandlers(p *pipeline.Pipeline) {
    b.addInitHandlers(p)        // 1-5: 初始化和数据获取
    b.addFilterHandlers(p)      // 6-10: 筛选和验证
    b.addCategoryHandlers(p)    // 11-17: 分类和SKU处理
    b.addImageHandlers(p)       // 18-21: 图片处理
    b.addContentHandlers(p)     // 22-29: 内容构建和优化
    b.addSubmitHandlers(p)      // 30-32: 提交和保存
}
```

**文件结构**：
```
platforms/temu/
  ├── pipeline.go              (主构建器)
  ├── pipeline_init.go         (初始化阶段)
  ├── pipeline_filter.go       (筛选阶段)
  ├── pipeline_category.go     (分类阶段)
  ├── pipeline_image.go        (图片阶段)
  ├── pipeline_content.go      (内容阶段)
  └── pipeline_submit.go       (提交阶段)
```

---

#### 2. 替换 interface{} 为 any
**问题**：使用过时的 `interface{}` 类型

**修改位置**：
- `platforms/temu/pipeline.go`: `TemuPipelineBuilder` 结构体
- `platforms/temu/processor.go`: `GetAmazonProcessor()` 返回值

**修改示例**：
```go
// 修改前
amazonProcessor interface{}

// 修改后
amazonProcessor any
```

---

#### 3. 消除重复的管道创建逻辑
**问题**：`TemuProcessor` 中有两处创建管道的代码

**方案**：
```go
// 统一使用一个方法创建管道
func (p *TemuProcessor) buildPipeline() *pipeline.Pipeline {
    openaiConfig := openai.NewClientConfig(...)
    builder := NewTemuPipelineBuilder(...)
    return builder.BuildPipeline()
}

// 在构造函数和 createDynamicPipeline 中都调用此方法
```

---

### P1 - 重要优化

#### 4. 改进错误处理机制
**问题**：使用字符串匹配判断错误类型，不够健壮

**方案**：
```go
// 定义标准错误变量
var (
    ErrProductNotFound    = errors.New("产品不存在")
    ErrProductOffline     = errors.New("产品已下架")
    ErrAuthExpired        = errors.New("认证已过期")
    ErrTooManyVariants    = errors.New("变体数量过多")
)

// 使用 errors.Is 判断
func IsRetryableError(err error) bool {
    if errors.Is(err, ErrProductNotFound) {
        return false
    }
    if errors.Is(err, ErrAuthExpired) {
        return false
    }
    // ...
}
```

---

#### 5. 优化日志级别
**问题**：大量 Info 级别日志可能影响性能

**方案**：
```go
// 调整日志级别
// Debug: 详细的调试信息（队列状态、任务详情）
// Info: 关键操作（任务开始/完成、状态变更）
// Warn: 警告信息（队列满、重试）
// Error: 错误信息（处理失败）

// 示例
logrus.Debugf("队列状态: %d/%d", queueSize, bufferSize)  // 改为 Debug
logrus.Infof("任务处理完成: ID=%s", taskID)              // 保持 Info
```

---

#### 6. 添加 Context 超时控制
**问题**：部分操作可能没有正确处理 Context 取消

**方案**：
```go
// 在每个 Handler 中检查 Context
func (h *SomeHandler) Handle(ctx *pipeline.TaskContext) error {
    select {
    case <-ctx.Context.Done():
        return fmt.Errorf("操作被取消: %w", ctx.Context.Err())
    default:
    }
    
    // 执行业务逻辑
    // ...
}
```

---

### P2 - 性能优化

#### 7. 优化任务去重机制
**问题**：使用 map 存储所有处理中的任务，可能占用大量内存

**方案**：
```go
// 使用 sync.Map 替代 map + RWMutex
type UnifiedTaskFetcher struct {
    processingTasks sync.Map // taskID -> submitTime
}

// 或使用 LRU 缓存限制大小
import "github.com/hashicorp/golang-lru"

processingTasks, _ := lru.New(10000) // 最多缓存 10000 个任务
```

---

#### 8. 批量更新任务状态
**问题**：每个任务单独更新状态，可能产生大量 API 调用

**方案**：
```go
// 收集需要更新的任务
type TaskStatusUpdate struct {
    TaskID int64
    Status int16
    Error  string
}

// 批量更新（每 100 个或每 5 秒）
func (h *TaskHandler) batchUpdateTaskStatus(updates []TaskStatusUpdate) {
    // 调用批量更新 API
}
```

---

#### 9. 添加 Metrics 监控
**问题**：缺少性能指标监控

**方案**：
```go
// 添加 Prometheus metrics
var (
    taskProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "temu_task_processing_duration_seconds",
            Help: "任务处理耗时",
        },
        []string{"platform", "status"},
    )
    
    taskQueueSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "temu_task_queue_size",
            Help: "任务队列大小",
        },
        []string{"platform"},
    )
)
```

---

## 📁 建议的文件结构

```
platforms/temu/
├── pipeline/                    # 管道相关（新增目录）
│   ├── builder.go              # 主构建器
│   ├── stage_init.go           # 初始化阶段
│   ├── stage_filter.go         # 筛选阶段
│   ├── stage_category.go       # 分类阶段
│   ├── stage_image.go          # 图片阶段
│   ├── stage_content.go        # 内容阶段
│   └── stage_submit.go         # 提交阶段
├── handlers/                    # 处理器（保持不变）
├── types/                       # 类型定义（保持不变）
├── errors.go                    # 错误定义（优化）
├── processor.go                 # 处理器（优化）
├── task_handler.go              # 任务处理器（优化）
└── task_submitter.go            # 任务提交器（保持不变）
```

---

## 🚀 实施步骤

### 第一阶段：代码重构（1-2天）
1. 拆分 pipeline.go
2. 替换 interface{} 为 any
3. 消除重复代码

### 第二阶段：错误处理优化（1天）
4. 改进错误处理机制
5. 优化日志级别
6. 添加 Context 超时控制

### 第三阶段：性能优化（1-2天）
7. 优化任务去重机制
8. 批量更新任务状态
9. 添加 Metrics 监控

---

## ✅ 验证标准

1. **代码质量**
   - 所有文件不超过 300 行
   - 无 interface{} 使用
   - 无重复代码

2. **错误处理**
   - 使用标准错误变量
   - 使用 errors.Is/As 判断
   - 所有错误都有上下文信息

3. **性能**
   - 日志级别合理
   - Context 正确处理
   - 内存占用稳定

4. **可维护性**
   - 文件结构清晰
   - 职责分离明确
   - 注释完整

---

## 📊 预期收益

1. **代码质量提升 30%**
   - 文件更小，更易维护
   - 职责更清晰

2. **性能提升 10-20%**
   - 减少不必要的日志
   - 优化内存使用
   - 批量操作减少 API 调用

3. **可维护性提升 40%**
   - 模块化设计
   - 标准化错误处理
   - 完善的监控指标

---

## 🔗 相关文档

- [Go 代码生成最佳实践规则](../rules.md)
- [错误处理指南](./ERROR_HANDLING.md)
- [日志规范](./LOGGING_GUIDE.md)
