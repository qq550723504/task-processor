# 问题二：productjson 包结构违反 Go 编码规范

**严重程度**：高

## 问题描述

`internal/app/productjson/` 包含 21 个文件，`internal/domain/productjson/` 只有 2 个文件。
这种 `app/` + `domain/` 的分层结构本身就违反了 Go 按功能分组的规范——`domain` 是技术层概念，
和 `models/`、`services/` 是同一类问题。同时，核心业务逻辑全部堆在 `app/productjson/` 中，
而 `domain/productjson/` 形同虚设，造成了人为的包分裂。

## 违反 Go 编码规范的具体问题

### 1. 用技术层目录分割了同一业务包

规范要求**按功能分组**，每个包内包含自己的模型和逻辑。
`app/productjson/` 和 `domain/productjson/` 本质上是同一个业务包被人为拆成两半，
`domain` 这个包名和 `models/`、`services/` 一样，是技术层概念而非业务概念。

当前 `app/productjson/` 内部混杂了三类职责，但它们都应该在同一个 `productjson/` 包里：

| 文件 | 实际职责 | 目标位置 |
|------|----------|---------|
| `scorer.go` | 质量评分算法（权重计算、LLM 融合） | `productjson/` |
| `llm_scorer.go` | LLM 评分器（Prompt 构建、评分解析、评分融合） | `productjson/` |
| `strategy.go` | 处理策略选择（分数阈值 → 策略映射） | `productjson/` |
| `result_validator.go` | 结果验证（关键词匹配、完整性检查） | `productjson/` |
| `validator.go` | 输入验证（图片 URL 校验、文本分析） | `productjson/` |
| `suggester.go` | 增强建议生成（基于验证结果的业务规则） | `productjson/` |
| `understanding.go` | 产品多模态分析（LLM 图文融合） | `productjson/` |
| `variant.go` | 变体/规格生成（LLM 驱动的业务逻辑） | `productjson/` |
| `service.go` / `service_*.go` | 应用编排（任务调度、HTTP 处理） | `productjson/` |
| `handler.go` | HTTP Handler | `productjson/` |
| `parser.go` | 输入解析（URL 抓取触发） | `productjson/` |
| `response.go` | HTTP 响应结构体 | `productjson/` |
| `config.go` | 配置 | `productjson/` |
| `model.go` | 领域模型（当前在 `domain/productjson/`） | `productjson/` |
| `repository.go` | 数据访问接口（当前在 `domain/productjson/`） | `productjson/` |

### 2. 接口设计违反小接口原则

规范要求接口保持精简（通常 1-3 个方法）。当前多个接口方法数量超标：

```go
// InputValidator 有 4 个方法，违反小接口原则
type InputValidator interface {
    Validate(...)
    ValidateImages(...)
    ValidateText(...)
    ValidateScrapedData(...)
}

// ResultValidator 有 4 个方法
type ResultValidator interface {
    ValidateResult(...)
    CheckImageConsistency(...)
    CheckKeywordMatch(...)
    CheckCompleteness(...)
}
```

这些接口应按职责拆分为更小的单一职责接口，或将辅助方法改为包级函数（不导出）。

### 3. 单包文件数量超标，职责不单一

规范要求"一个文件只做一类事"，单文件建议不超过 500 行。
`app/productjson/` 有 21 个文件，`scorer.go` 在同一文件中包含图片评分、文本评分、
抓取数据评分三种不同职责；`validator.go` 混合了 URL 格式校验（基础设施）和
文本关键词提取（领域逻辑）两类关注点。

### 4. 构造函数参数过度使用 Config 结构体

规范要求"禁止预留扩展性"，不要为未来需求编写复杂工厂模式。
当前多个构造函数使用了 `XxxConfig` 结构体包装参数，但实际字段很少：

```go
// QualityScorerConfig 只有 6 个字段，直接传参更地道
type QualityScorerConfig struct {
    ImageWeight   float64
    TextWeight    float64
    ScrapedWeight float64
    LLMScorer     LLMScorer
    EnableLLM     bool
    Metrics       MetricsCollector
}
```

## 影响分析

1. **违反分层原则**：`domain/productjson` 包存在但是空洞的，真正的领域逻辑全在 `app/productjson`，
   分层形同虚设，新开发者无法从目录结构理解业务边界。
2. **可复用性差**：评分算法、策略选择等逻辑如果需要在其他地方使用（如批量处理、定时任务），
   必须依赖整个 `app/productjson` 包，连带引入 HTTP Handler、任务队列等无关依赖。
3. **测试困难**：领域逻辑混在应用层，单元测试需要构造大量应用层依赖（Worker 池、HTTP 客户端等），
   而这些依赖本应与业务规则无关。
4. **依赖方向倒置**：`domain` 层的逻辑依赖 `app` 层的接口定义，违反了依赖倒置原则，
   导致无法单独编译或测试 domain 层。

## 重构建议

按职责将 `app/productjson` 拆分，领域逻辑下沉到 `domain/productjson`：

```
internal/domain/productjson/
    model.go           ← 已有，保留
    repository.go      ← 已有，保留
    interfaces.go      ← 新增：LLMManager、LLMClient 等核心接口（从 app 层迁移）
    scorer.go          ← 新增：质量评分逻辑（从 app/scorer.go 迁移）
    llm_scorer.go      ← 新增：LLM 评分器（从 app/llm_scorer.go 迁移）
    strategy.go        ← 新增：策略选择逻辑（从 app/strategy.go 迁移）
    validator.go       ← 新增：输入验证业务规则（从 app/validator.go 迁移）
    result_validator.go← 新增：结果验证逻辑（从 app/result_validator.go 迁移）
    suggester.go       ← 新增：增强建议生成（从 app/suggester.go 迁移）
    understanding.go   ← 新增：产品多模态分析（从 app/understanding.go 迁移）
    variant.go         ← 新增：变体/规格生成（从 app/variant.go 迁移）

internal/app/productjson/
    service.go         ← 保留：应用编排（调用 domain 层、管理任务队列）
    service_task.go    ← 保留：任务管理
    service_process.go ← 保留：处理流程编排
    service_helpers.go ← 保留：应用层辅助函数
    handler.go         ← 保留：HTTP Handler
    parser.go          ← 保留：输入解析（触发 WebScraper）
    response.go        ← 保留：HTTP 响应结构体
    config.go          ← 保留：应用层配置
    interfaces.go      ← 精简：只保留 RedisClient、WebScraper 等基础设施接口
    llm_score_cache.go ← 保留：缓存实现（基础设施）
    validation_cache.go← 保留：缓存实现（基础设施）
    generator.go       ← 保留：生成器编排
    generator_json.go  ← 保留：JSON 生成编排
```

**判断标准**：一段代码是否属于领域层，看它是否包含业务规则。
- "分数 >= 80 用完整策略" → 业务规则，属于 `domain/`
- "图片数量 >= 5 得 100 分" → 业务规则，属于 `domain/`
- "调用 domain.Score()，然后调用 domain.SelectStrategy()，写入 Redis" → 应用编排，属于 `app/`
- "发 HTTP 请求验证图片 URL 可访问性" → 基础设施，接口定义在 `domain/`，实现在 `app/` 或 `infra/`

## 影响分析

1. **包名违规**：`domain` 是技术层概念，对包的使用者没有业务信息量，违反"包名应描述功能"的规范。
2. **人为包分裂**：同一业务的模型和逻辑被拆在两个包里，新开发者需要同时打开两个目录才能理解完整逻辑。
3. **测试困难**：`app/productjson/` 中的业务逻辑（评分、策略）和基础设施（HTTP Handler、任务队列）混在一起，单元测试需要构造大量无关依赖。
4. **接口臃肿**：多个接口超过 3 个方法，难以 mock，测试成本高。

## 重构建议

将 `internal/app/productjson/` 和 `internal/domain/productjson/` 合并为 `internal/productjson/`，
文件命名保持不变，只修改 `package` 声明和 import 路径：

```
internal/productjson/
    model.go            ← 来自 domain/productjson/model.go
    repository.go       ← 来自 domain/productjson/repository.go
    interfaces.go       ← 来自 app/productjson/interfaces.go
    scorer.go
    llm_scorer.go
    llm_score_cache.go
    strategy.go
    validator.go
    validation_cache.go
    result_validator.go
    suggester.go
    understanding.go
    variant.go
    parser.go
    generator.go
    generator_json.go
    service.go
    service_task.go
    service_process.go
    service_helpers.go
    handler.go
    response.go
    config.go
```

**何时考虑拆子包**：当某个子逻辑（如评分）被 `productjson` 以外的业务包复用时，
再将其提取为独立包（如 `internal/scoring/`）。在此之前，单包 + 文件按职责命名已经足够清晰。
