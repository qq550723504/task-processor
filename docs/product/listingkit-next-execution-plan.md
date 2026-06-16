# ListingKit 下一阶段执行计划

## 目的

这份文档把接下来一段时间的工作从“产品路线图”落成可执行计划。

当前策略不是先做一次大重构，也不是继续无边界增加新功能，而是：

```text
以 SHEIN 主链路稳定为牵引开发能力；
每补一个关键能力，就顺手切清一块模块边界；
先让运营可独立稳定使用，再扩展其他平台。
```

## 总体优先级

| 优先级 | 目标 | 判断标准 |
| --- | --- | --- |
| P0 | SHEIN 主链路稳定 | 一个运营人员能独立从来源素材 / SDS 商品走到保存草稿或发布，不需要工程师解释状态或查日志。 |
| P1 | 运营效率和可诊断性 | 运营负责人能批量找到失败、阻断、可提交、需重试任务，并能做日常复盘。 |
| P2 | 多平台产品化 | 把 SHEIN 跑通的状态、阻断、提交、恢复模型迁移到 TEMU、Amazon、Walmart。 |

## 执行原则

1. 不先做大范围目录搬迁。
2. 不再把新的平台规则塞进 `internal/listingkit` 根包。
3. 每个 P0 能力都必须有真实任务验收记录。
4. 每个失败场景都必须能说明：用户能做什么、何时需要工程介入、要记录哪些字段。
5. 每次重构必须小步、可回滚，并在 PR 里说明是否改变行为。

## 阶段 0：安全网和边界规则

时间：2 到 3 天。

### 要做

```bash
go test ./...
go test ./... -coverprofile=docs/refactoring/coverage-baseline.out
go list ./... > docs/refactoring/packages-baseline.txt
go mod graph > docs/refactoring/mod-graph-baseline.txt
```

同时确认或补充：

- `docs/architecture/project-boundaries.md`
- `docs/refactoring/test-baseline.txt`
- `docs/refactoring/packages-baseline.txt`
- `docs/refactoring/mod-graph-baseline.txt`
- `docs/refactoring/coverage-baseline.out`

### 验收标准

- 当前测试结果被记录。
- 当前包列表和依赖图被记录。
- 后续新能力的包归属有明确依据。
- 没有在建立 baseline 前做大范围文件移动。

## 阶段 1：真实接口验收和错误恢复 SOP

时间：1 周。

### 要做

1. 使用 [`validation/listingkit-real-api-validation-report-template.md`](./validation/listingkit-real-api-validation-report-template.md) 记录每轮真实任务联调。
2. 使用 [`ops/listingkit-error-recovery-sop.md`](./ops/listingkit-error-recovery-sop.md) 固化失败恢复方式。
3. 至少沉淀：
   - 1 条真实成功任务。
   - 1 条真实失败并恢复或明确不可恢复的任务。

### 记录范围

- task_id
- 输入来源和输入内容摘要
- 状态流转
- workspace payload 摘要
- readiness 结果和 blocker keys
- 保存草稿 / 发布请求与响应
- 失败阶段和恢复动作
- 未知状态、未知 blocker、空错误响应

### 验收标准

- 至少一条真实任务从创建到保存草稿或发布完整通过。
- 至少一条失败路径能被 UI 理解并进入恢复流程。
- 没有未记录的未知任务状态。
- 没有未记录的未知 readiness blocker key。

## 阶段 2：SHEIN 提交状态机和幂等保护

时间：2 到 3 周。

### 目标

把保存草稿 / 发布从黑盒动作变成可观察、可恢复、幂等的提交过程。

### 推荐阶段

```text
validate
  -> prepare_product
  -> upload_images
  -> pre_validate
  -> submit_remote
  -> persist_result
```

### 推荐模型

```text
SubmitAttempt
SubmitAction
SubmitPhase
SubmitStatus
SubmitError
SubmitRemoteRecord
SubmitIdempotencyKey
```

核心字段建议：

```text
attempt_id
task_id
tenant_id
target_platform
action: save_draft / publish
status: pending / running / succeeded / failed / recovering
phase: validate / prepare_product / upload_images / pre_validate / submit_remote / persist_result
idempotency_key
remote_product_id
remote_draft_id
remote_publish_id
error_code
error_message
recoverable
created_at
updated_at
finished_at
```

### 模块边界

```text
internal/listing/submission
  通用提交状态机、attempt、幂等、阶段流转、恢复入口

internal/marketplace/shein/publishing
  SHEIN 远端提交、图片上传、pre-validate、保存草稿、发布规则

internal/listingkit
  只保留提交入口 facade，兼容旧 API 并做高层编排

internal/app/httpapi
  只做 handler / route / service wiring
```

### 验收标准

- 保存草稿或发布进行中时，UI 能显示当前阶段。
- 失败后能看到失败阶段和原因。
- 同一 `idempotency_key` 重放不会重复提交。
- 同一 task 同一 action 并发提交不会重复调用 SHEIN 远端接口。
- 至少一条真实任务保存草稿通过。
- 至少一条真实任务发布通过。

## 阶段 3：readiness blocker taxonomy

时间：1 到 2 周。

### 目标

让每个阻断项都能稳定映射到用户可理解的修复动作。

### 推荐字段

```text
blocker_key
severity: blocker / warning / info
domain: category / attribute / sale_attribute / image / price / sku / store / remote / system
repair_target
repair_route
message_template
recoverable
requires_engineering
```

### 常见映射

| blocker | 修复区域 |
| --- | --- |
| `missing_category` | 类目修复区 |
| `missing_required_attribute` | 普通属性区 |
| `missing_sale_attribute` | 销售属性区 |
| `image_upload_failed` | 图片区 |
| `price_invalid` | 价格区 |
| `sku_invalid` | SKU 区 |
| `shein_remote_validation_failed` | 提交报告区 |
| `unknown` | 阻断项详情兜底区 |

### 验收标准

- 所有已知 blocker 都能跳到修复区域。
- 未知 blocker 有可理解兜底展示。
- 未知 blocker 会进入真实接口验收报告。
- readiness 一次通过率可以开始统计。

## 阶段 4：任务列表和队列页运营化

时间：2 周。

### 任务列表能力

- 平台筛选。
- 来源筛选。
- 任务状态筛选。
- readiness 状态筛选。
- 提交状态筛选。
- 阻断类型筛选。
- 创建时间 / 更新时间排序。
- 每行显示下一步动作。

### next action 示例

```text
进入工作台
修复阻断项
查看提交状态
重试保存草稿
重试发布
查看失败原因
进入队列排查
```

### 队列页语义

| 动作 | 含义 |
| --- | --- |
| Review | 人工检查资料或阻断项。 |
| Retry | 可重试的系统动作。 |
| Inspect | 需要查看详情，可能需要工程介入。 |

### 验收标准

- 运营能从列表快速找到当天失败任务。
- 运营能从列表快速找到可提交任务。
- 不需要复制 task_id 到其他页面才能继续处理。
- 队列页能支持每日失败任务复盘。

## 阶段 5：设置页配置健康检查

时间：1 周。

### 检查项

- AI client 连通性。
- SHEIN 店铺 token、权限、类目接口。
- SDS 接口或登录态。
- 图片模型配置完整性。
- 价格规则完整性。
- 对象存储可用性。
- 队列 / Worker 基础可用性。

### 显示影响范围

| 配置问题 | 影响 |
| --- | --- |
| AI client 不可用 | 标题、卖点、描述、属性改写失败。 |
| SHEIN token 失效 | 保存草稿和发布失败。 |
| SDS 登录态失效 | SDS 商品库和 Studio 创建任务失败。 |
| 图片模型缺失 | 主图、白底图、mockup 生成失败。 |
| 价格规则缺失 | readiness price blocker。 |

### 验收标准

- 新建任务前可以确认关键配置是否可用。
- 配置错误能在设置页暴露。
- 不用等任务失败后才发现 token、权限、模型、SDS 登录态问题。

## 阶段 6：SHEIN Studio 批量任务效率

时间：1 到 2 周。

### 要做

- 批量任务按可提交、需修复、处理中、生成失败、提交失败、已保存草稿、已发布分组。
- 批量操作支持部分成功。
- 失败项可以单独重试。
- 成功项不重复提交。
- 批量结果回流任务列表。

### 验收标准

- 一批任务中部分失败不会阻断其他可提交任务。
- 用户能清楚知道哪些任务需要单独进入工作台。
- 批量保存草稿失败时可以只重试失败项。
- 成功项不会重复提交。

## 阶段 7：TEMU / Amazon / Walmart 产品化

在 SHEIN 主链路稳定后启动。

### TEMU

先回答：

- 是否进入 ListingKit 工作台？
- 是否支持人工审核？
- 是否支持保存草稿 / 发布？
- 有哪些 blocker？
- 失败后如何恢复？

### Amazon

先回答：

- Amazon ListingKit 是资料包生成还是完整工作台？
- `amazonlisting` 专用工作台和 ListingKit 多平台包怎么分工？
- 用户入口是哪个？
- 是否支持提交？

### Walmart

先回答：

- 近期是否进入人工审核和提交流程？
- 如果不进入，是否只标记为资料包生成目标？
- UI 是否需要明确说明能力范围？

## 10 周排期建议

| 周期 | 重点 | 主要交付 |
| --- | --- | --- |
| Week 1 | 安全网 + 验收模板 + SOP 骨架 | baseline、边界确认、模板、SOP 骨架、1 条真实任务记录。 |
| Week 2 | 真实链路闭环验证 | 成功任务、失败任务、未知状态 / blocker 清单。 |
| Week 3-4 | SHEIN 提交状态机后端 | SubmitAttempt、SubmitPhase、idempotency、状态查询、模块边界切分。 |
| Week 5 | 提交状态 UI + 真实验证 | 阶段展示、失败原因、真实保存草稿 / 发布验证。 |
| Week 6 | readiness blocker taxonomy | blocker 分类、修复跳转、unknown 兜底。 |
| Week 7-8 | 任务列表 / 队列页运营化 | 筛选、next action、Review / Retry / Inspect、失败复盘字段。 |
| Week 9 | 设置页健康检查 | AI、SHEIN、SDS、图片、价格配置诊断。 |
| Week 10 | SHEIN Studio 批量效率 | 状态分组、部分成功、失败项重试、批量结果回流。 |

## 暂缓事项

- 暂缓完整 TEMU / Amazon / Walmart 工作台。
- 暂缓大规模目录重命名。
- 暂缓继续增加大量 AI 文案生成能力，除非它直接影响 SHEIN 主链路验收。
- 暂缓把新的平台规则加到 `internal/listingkit` 根包。

## 推荐 Epic

具体拆分见 [`backlog/listingkit-execution-issues.md`](./backlog/listingkit-execution-issues.md)。
