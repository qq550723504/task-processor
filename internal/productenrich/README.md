# productenrich

`productenrich` 负责商品信息富化。

它的目标是把输入的图片、文本描述、1688 商品详情页信息，整理成结构化商品结果，供后续上架、内容生成或映射流程使用。

## 主要能力

- 解析输入
  - 接收 `image_urls`
  - 接收 `text`
  - 接收 1688 商品详情页 `product_url`
- 校验输入质量
- 分析商品图片和文本
- 生成结构化商品 JSON
- 生成规格、变体、卖点等信息
- 校验结果完整性和一致性
- 支持异步任务、状态机、重试和降级策略

## 主流程

`productenrich` 当前的主处理流程是：

```text
parse_input
  -> validate_strategy
  -> analyze_product
  -> generate_json
  -> validate_result
```

处理入口在：

- [service_process.go](/D:/code/task-processor/internal/productenrich/service_process.go)
- [pipeline.go](/D:/code/task-processor/internal/productenrich/pipeline.go)

异步执行和重试在：

- [pipeline/processor.go](/D:/code/task-processor/internal/productenrich/pipeline/processor.go)
- [pipeline/state_machine.go](/D:/code/task-processor/internal/productenrich/pipeline/state_machine.go)

## 目录职责

- [api](/D:/code/task-processor/internal/productenrich/api)
  HTTP handler 层，负责请求绑定、错误码映射、响应返回

- [enrich](/D:/code/task-processor/internal/productenrich/enrich)
  真正的富化实现层，包括：
  - 输入解析
  - 商品理解
  - JSON 生成
  - 变体生成
  - 1688 抓取适配

- [pipeline](/D:/code/task-processor/internal/productenrich/pipeline)
  异步执行层，包括：
  - processor
  - 状态机
  - 重试判定

- [store](/D:/code/task-processor/internal/productenrich/store)
  仓储实现层，包括内存仓储和数据库仓储实现

## 关键文件

- [model.go](/D:/code/task-processor/internal/productenrich/model.go)
  领域模型和核心数据结构

- [service.go](/D:/code/task-processor/internal/productenrich/service.go)
  service 配置和能力边界

- [service_task.go](/D:/code/task-processor/internal/productenrich/service_task.go)
  创建任务、查询任务结果

- [service_process.go](/D:/code/task-processor/internal/productenrich/service_process.go)
  主处理编排入口

- [pipeline.go](/D:/code/task-processor/internal/productenrich/pipeline.go)
  stage runner 和流程阶段定义

- [result_validator.go](/D:/code/task-processor/internal/productenrich/result_validator.go)
  结果完整性和质量校验

## 当前特点

- 已拆分成 `api / enrich / pipeline / store` 几层
- 使用显式状态机控制任务流转
- 使用显式 pipeline 控制业务处理阶段
- 支持 `compat / strict` 两种 capability 模式
- 更适合做“商品理解和结构化输出”，不负责真实图片编辑

## 适用场景

- 从 1688 商品页提取商品基础信息
- 把图片和文本输入整理成标准商品 JSON
- 给 Amazon、Temu、Shein 等后续流程提供统一商品上下文

## 与 productimage 的边界

`productenrich` 和 `productimage` 有数据衔接，但职责需要明确分开。

### 应留在 productenrich 的能力

- 输入解析标准
- `image_urls / text / product_url` 规则
- 1688 商品抓取
- 商品图片与文本理解
- 多模态融合
- 商品属性、卖点、规格、变体生成
- 商品结构化 JSON 输出

也就是：
`productenrich` 负责“懂商品，并产出结构化商品数据”。

### 不应继续长进 productenrich 的能力

- 白底图生成
- 主图优化
- 图片发布
- 图片审核状态
- 图片资产生命周期
- Amazon 图片质量评分

这些能力应放在 `productimage`。

### 与 productimage 的关系

推荐长期保持：

```text
productenrich
  -> ParsedInput
  -> ProductAnalysis
  -> ProductJSON

productimage
  -> 消费 ParsedInput / ProductAnalysis
  -> 产出 ImageProcessResult / ImageAsset
```

也就是：
- `productenrich` 是商品理解上游
- `productimage` 是图片资产下游
