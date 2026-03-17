# 包组织重构方案

> 基于 `.kiro/steering/go-coding-standards.md` 规范，对当前项目包组织问题的系统性重构计划。

## 问题总览

通过代码审查，发现以下六类系统性问题：

1. **`internal/domain/` — DDD 残留**：`domain` 是技术层概念，内部又出现 `service/`、`repository/` 二次按层分组
2. **`internal/shein/` 根目录散落**：12 个职责各异的文件堆在根目录，父包依赖子包形成反向依赖
3. **`internal/amazon/core/service/` — 技术层分组**：schema、llm、image、s3 全混在一个 `service/` 包
4. **`internal/app/` 过度拆分**：`di/` 包实现了完整 IoC 容器（含反射），`messaging/` 职责混乱
5. **错误处理三套并存**：`core/errors`、`pkg/apperr`、各平台自定义，互相重叠
6. **跨平台命名不一致**：`shein/taskexecutor/` vs `temu/scheduler/`，同一职责两个名字

---

## 目标架构

```
internal/
├── model/                   # 【迁移】纯共享数据模型（原 domain/model/）
│   ├── task.go
│   ├── product.go
│   └── task_status.go
│
├── core/                    # 核心基础设施（基本不变）
│   ├── config/
│   ├── errors/              # 【合并】吸收 pkg/apperr，成为唯一错误包
│   ├── lifecycle/
│   ├── logger/
│   ├── metrics/
│   └── system/
│
├── app/                     # 应用层（精简）
│   ├── bootstrap/           # 【简化】去掉对 di/ 的依赖，直接手动组装
│   ├── scheduler/
│   ├── task/                # 【合并】吸收 domain/task 的运行时组件
│   ├── state/
│   ├── runner/
│   └── updater/
│
├── product/                 # 【迁移】产品获取功能（原 domain/product/）
│   ├── fetcher.go
│   ├── cache.go
│   ├── repository.go
│   └── types.go
│
├── infra/                   # 基础设施（基本不变）
│   ├── auth/
│   ├── clients/
│   ├── database/
│   ├── httpx/
│   ├── lock/
│   ├── monitoring/
│   ├── productcrawler/
│   ├── rabbitmq/            # 【吸收】domain/queue/naming.go
│   ├── repository/
│   ├── storage/             # 【新】s3_uploader 从 amazon/core/service 归位
│   └── worker/
│
├── pipeline/                # 通用 Pipeline（不变）
├── crawler/                 # 爬虫（不变）
│
├── amazon/                  # Amazon 平台
│   ├── api/                 # 不变
│   ├── attribute/           # 不变
│   ├── schema/              # 【新】从 core/service/schema_*.go 迁移
│   ├── llm/                 # 【新】从 core/service/llm_*.go 迁移
│   ├── image/               # 【新】从 core/service/image_*.go 迁移
│   ├── converter.go         # 从 core/ 上移
│   ├── identifier.go        # 从 core/ 上移
│   ├── validator.go         # 从 core/ 上移
│   └── processor.go
│
├── shein/                   # SHEIN 平台
│   ├── context/             # 【新】TaskContext（参考 temu/context 的做法）
│   ├── api/                 # 不变
│   ├── category/            # 不变
│   ├── client/              # 不变
│   ├── content/             # 【清理】文件名去掉 _service 后缀
│   ├── product/             # 不变
│   ├── publish/             # 【清理】文件名去掉 _service 后缀
│   ├── store/               # 【清理】文件名去掉 _service 后缀
│   ├── translate/           # 不变
│   ├── validation/          # 不变
│   ├── pricing/             # 【扩充】吸收 operation/pricing_*.go
│   ├── inventory/           # 【新】从 operation/inventory_sync_*.go 拆出
│   ├── activity/            # 【新】从 operation/activity_*.go 拆出
│   ├── productsync/         # 【新】从 operation/product_sync_*.go 拆出
│   ├── mapping/             # 【新】从 operation/mapping_repair_*.go 拆出
│   ├── pipeline/            # 不变
│   └── scheduler/           # 【重命名】从 taskexecutor/ 改名，统一与 temu 一致
│
├── temu/                    # TEMU 平台（基本不变）
│   ├── scheduler/           # 不变（已是正确命名）
│   └── ...
│
├── taskbase/                # 跨平台任务基类（不变）
├── platformbase/            # 平台工厂基类（不变）
├── pricing/                 # 通用定价（不变）
├── productenrich/           # 商品增强（不变）
│
└── pkg/                     # 内部工具库（保留位置，清理内容）
    ├── hashx/
    ├── strx/
    └── ...（删除 apperr/，其余保留）
```

---

## Phase 1：命名统一与文件规范

**风险：低 | 预计耗时：1-2 天**

不改变任何逻辑，只做重命名。每个小步骤单独 commit。

### 1.1 统一平台调度包命名

`shein/taskexecutor/` 和 `temu/scheduler/` 是同一职责，命名不一致。

```
internal/shein/taskexecutor/ → internal/shein/scheduler/
```

使用 `smartRelocate` 移动目录，import 路径自动更新。类型名 `SheinTaskFactory` 保持不变。

### 1.2 去掉文件名的 `_service` 后缀

Go 文件名应描述内容，不需要 `_service` 这种技术层后缀。

**`shein/content/`**
```
config_service.go          → config.go
optimizer_service.go       → optimizer.go
processor_service.go       → processor.go
validator_service.go       → validator.go
word_service.go            → word.go
words_processor_service.go → words_processor.go
utils_service.go           → utils.go
```

**`shein/publish/`**
```
checker_service.go          → checker.go
error_handler_service.go    → error_handler.go
exists_check_service.go     → exists_check.go
handler_service.go          → handler.go
result_service.go           → result.go
saver_service.go            → saver.go
validator_service.go        → validator.go
variant_success_service.go  → variant_success.go
```

**`shein/store/`**
```
site_service.go       → site.go
store_id_service.go   → store_id.go
store_info_service.go → store_info.go
supplier_service.go   → supplier.go
warehouse_service.go  → warehouse.go
```

**`shein/pipeline/`**
```
error_service.go     → error.go
pipeline_service.go  → pipeline.go
processor_service.go → processor.go
router_service.go    → router.go
status_service.go    → status.go
submitter_service.go → submitter.go
task_service.go      → task.go
```

### 1.3 合并重复的错误包

`internal/pkg/apperr/apperr.go` 和 `internal/core/errors/errors.go` 定义了两个 `AppError`，常量名重叠（`ErrCodeConfig`、`ErrCodeAuth`、`ErrCodeValidation`）。以 `core/errors` 为准（实现更完整）。

操作步骤：
1. 全局搜索 `"task-processor/internal/pkg/apperr"` 的所有 import
2. 逐一替换为 `"task-processor/internal/core/errors"`，调整调用方式
3. `go build ./...` 验证编译通过
4. 删除 `internal/pkg/apperr/`

---

## Phase 2：包拆分重组

**风险：中 | 预计耗时：3-5 天**

解决职责混乱的包。每次只移动一个子域，保证编译通过后再移下一个。

### 2.1 拆分 `internal/shein/operation/`

当前 `operation/` 有 40+ 个文件，混合了五个完全不同的业务域。

**库存同步（14 个文件）→ `shein/inventory/`**
```
inventory_sync.go              → sync.go
inventory_sync_api.go          → api.go
inventory_sync_strategy.go     → strategy.go
inventory_sync_config_getter.go → config.go
inventory_sync_monitor.go      → monitor.go
inventory_sync_record.go       → record.go
inventory_sync_helper.go       → helper.go
inventory_sync_types.go        → types.go
inventory_sync_cost_calculator.go → cost_calculator.go
inventory_sync_change_checker.go  → change_checker.go
inventory_sync_price_strategy.go  → price_strategy.go
inventory_sync_amazon_fetcher.go  → amazon_fetcher.go
```

**活动报名（7 个文件）→ `shein/activity/`**
```
activity_registration.go             → registration.go
activity_config.go                   → config.go
activity_errors.go                   → errors.go
activity_time_limited_discount.go    → time_limited.go
activity_registration_mixed.go       → mixed.go
activity_registration_profit.go      → profit.go
activity_registration_config.go      → registration_config.go
```

**商品同步（4 个文件）→ `shein/productsync/`**
```
product_sync.go         → service.go
product_sync_fetcher.go → fetcher.go
product_sync_enricher.go → enricher.go
product_sync_types.go   → types.go
```

**映射修复（5 个文件）→ `shein/mapping/`**
```
mapping_repair_service.go    → service.go
mapping_repair_handler.go    → handler.go
mapping_repair_builder.go    → builder.go
mapping_repair_strategies.go → strategies.go
mapping_repair_types.go      → types.go
```

**核价（剩余文件）→ 合并到已有的 `shein/pricing/`**
```
auto_pricing.go       → auto_pricing.go
pricing_builder.go    → builder.go
pricing_calculator.go → 合并到已有 calculator.go
pricing_evaluator.go  → evaluator.go
price_calculator.go   → price_calculator.go
product_data_helper.go → product_data.go
marketing_repo.go     → marketing_repo.go
validation_utils.go   → validation.go
```

### 2.2 解决 `shein/` 根目录散落文件

核心问题：`types.go` 里的 `TaskContext` 依赖了所有子包的具体类型，导致父包依赖子包。参考 `temu/context/` 的已有设计。

**新建 `internal/shein/context/` 包：**

```
shein/context/
    context.go    ← TaskContext 定义（从 shein/types.go 迁移）
    attribute.go  ← 属性相关类型（从 shein/attribute.go 迁移，30+ 个类型）
    models.go     ← EnrichedSkuInfo 等（从 shein/models.go 迁移）
```

同步修复 `GetTask()` 返回 `nil` 的未完成实现：

```go
// 修复前（shein/types.go）
func (ctx *TaskContext) GetTask() *types.Task {
    // 需要根据实际情况实现转换逻辑
    return nil
}

// 修复后（shein/context/context.go）
func (ctx *TaskContext) GetTask() *model.Task {
    return ctx.Task
}
```

同步修复 `types.go` 中同一包被 import 两次的问题：

```go
// 修复前
import (
    "task-processor/internal/domain/model"
    types "task-processor/internal/domain/model"  // 重复 import
    ...
)

// 修复后
import (
    "task-processor/internal/model"  // Phase 3 完成后的路径
    ...
)
```

**根目录剩余文件处理：**
```
shein/errors.go               → 保留（包级错误定义，合理）
shein/module_errors.go        → 合并到 shein/errors.go
shein/region.go               → 保留（地区常量，合理）
shein/sensitive_words_filter.go → 移到 shein/content/
shein/string_sanitizer.go     → 移到 shein/content/
shein/time_helper.go          → 移到 shein/content/（或删除，视使用情况）
shein/json_map.go             → 移到 shein/context/ 或 shein/product/
shein/inventory.go            → 移到 shein/inventory/
shein/product.go              → 移到 shein/product/
```

### 2.3 拆分 `internal/amazon/core/`

**Schema 相关 → `amazon/schema/`**
```
core/service/schema_fetcher.go  → schema/fetcher.go
core/service/schema_parser.go   → schema/parser.go
core/service/schema_builder.go  → schema/builder.go
core/service/schema_manager.go  → schema/manager.go
```

**LLM 相关 → `amazon/llm/`**
```
core/service/llm_attribute_mapper.go → llm/mapper.go
core/service/openai_llm_client.go    → llm/openai_client.go
```

**图片相关 → `amazon/image/`**
```
core/service/image_management.go      → image/management.go
core/service/image_processor.go       → image/processor.go
core/service/image_attribute_builder.go → image/attribute_builder.go
```

**S3 上传器 → `infra/storage/`（基础设施归位）**
```
core/service/s3_uploader.go → infra/storage/s3.go
```

**`service_factory.go` → 删除**

工厂逻辑分散到各自包的 `New` 函数，不需要统一工厂。

**`core/` 层级消除**
```
core/model/    → 评估后合并到 amazon/ 根目录或 internal/model/
core/handler/  → 合并到 amazon/ 根目录
core/converter.go        → amazon/converter.go
core/identifier_generator.go → amazon/identifier.go
core/validator.go        → amazon/validator.go
core/variant_extractor.go → amazon/variant_extractor.go
```

---

## Phase 3：核心架构重构

**风险：高 | 预计耗时：1-2 周**

影响面最广，需要充分的测试覆盖，建议作为专项重构 Sprint 执行。

### 3.1 消灭 `internal/domain/`

分四步执行，每步单独 commit 并验证编译。

**Step 1：迁移 `domain/model/` → `internal/model/`**

只改目录名，不改任何逻辑。

```bash
# 全局替换 import 路径
# "task-processor/internal/domain/model" → "task-processor/internal/model"
```

影响范围：几乎所有包都 import 了 `domain/model`，这是改动最大的一步。建议用 IDE 的全局重构功能或脚本批量替换，替换后立即 `go build ./...`。

**Step 2：迁移 `domain/task/` → `app/task/`**

```
domain/task/deduplicator.go    → app/task/deduplicator.go
    注意：app/task/ 已有 deduplication_manager.go，需要合并，二者功能重叠

domain/task/job.go             → app/task/job.go
domain/task/message_adapter.go → app/task/message_adapter.go
domain/task/crawler_task.go    → crawler/ 相关包（按内容决定归属）
domain/task/crawler_result.go  → crawler/ 相关包
domain/task/errors.go          → app/task/errors.go
domain/task/task_errors.go     → 合并到 app/task/errors.go（两个错误文件合一）
```

**Step 3：迁移 `domain/product/` → `internal/product/`**

```
internal/domain/product/ → internal/product/
```

直接改目录名，语义更清晰（"产品数据获取"功能，不是"领域层"）。

**Step 4：处理剩余 domain 子包**

```
domain/queue/naming.go    → infra/rabbitmq/naming.go（队列命名属于基础设施）
domain/message/types.go   → app/task/message_types.go
domain/validation/        → 评估：若只被 shein/temu 使用，移到对应平台包
```

完成后 `internal/domain/` 目录应为空，可以删除。

### 3.2 简化 `internal/app/di/`

当前 IoC 容器的问题：
- `GetByType` 使用反射，是规范明确反对的"黑魔法"
- 通过字符串 key 获取服务（`c.Get("config")`），类型不安全
- `ServiceRegistry`、`InstanceCache` 接口是为了"未来扩展"而预留的过度设计

**目标：** 用函数式依赖注入替代，在 `bootstrap/` 里直接手动组装。

```go
// 现在（通过容器，类型不安全）
configInstance, _ := c.Get("config")
cfg := configInstance.(*config.Config)  // 运行时类型断言

// 目标（直接传参，编译期类型安全）
func NewTemuProcessor(
    cfg *config.Config,
    logger *logrus.Logger,
    managementClient *management.ClientManager,
    amazonCrawler AmazonCrawler,
) (*Processor, error)
```

迁移步骤：
1. 在 `bootstrap/service_registry_simple.go` 里，逐个把 `container.Get()` 替换为直接构造函数调用
2. 把构造好的实例直接传给下游，而不是注册到容器
3. 当所有服务都不再通过容器获取时，删除 `di/` 包
4. `bootstrap/app.go` 里的 `container di.Container` 字段改为直接持有各服务实例

### 3.3 整理 `internal/app/messaging/`

`messaging/` 这个名字无法描述其 14 个文件的实际内容，按职责重新归位：

```
app/messaging/rabbitmq_service.go      → 评估是否移到 infra/rabbitmq/（基础设施）
app/messaging/http_servers.go          → app/bootstrap/ 或 infra/httpx/
app/messaging/platform_registry.go    }
app/messaging/processor_registry.go   } → app/registry/（新包，专门管注册表）
app/messaging/crawler_registry.go     }
app/messaging/shutdown_coordinator.go → app/bootstrap/
app/messaging/task_handler.go         → app/task/
app/messaging/task_submitter.go       → app/task/
app/messaging/queue_config.go         → infra/rabbitmq/
app/messaging/queue_initializer.go    → infra/rabbitmq/
app/messaging/result_reporter.go      → app/task/
app/messaging/service_manager.go      → app/bootstrap/
app/messaging/components.go           → app/bootstrap/
app/messaging/rabbitmq_publisher_adapter.go → infra/rabbitmq/
```

---

## 执行原则

### 每步操作的标准流程

```
1. 新建目标包，写好 package 声明
2. 移动文件（优先用 smartRelocate，自动更新 import）
3. go build ./... 验证编译
4. go test ./... 验证测试
5. 单独 commit
```

**commit message 格式：**
```
refactor: move shein/taskexecutor to shein/scheduler
refactor: split shein/operation/inventory into shein/inventory
refactor: merge pkg/apperr into core/errors
```

### 禁止同时做的事

- 移动文件的同时修改逻辑（移动和修改必须分开 commit）
- 一次移动多个不相关的包
- Phase 1/2 期间动 `domain/model/`（影响面太大，留到 Phase 3）

### 过渡期类型别名

如果某个包改动影响面太大，可以用类型别名过渡，等所有调用方更新后再删除旧包：

```go
// internal/shein/taskexecutor/compat.go（过渡期临时文件）
package taskexecutor

import "task-processor/internal/shein/scheduler"

// Deprecated: 请使用 shein/scheduler.SheinTaskFactory
type SheinTaskFactory = scheduler.SheinTaskFactory
```

---

## 优先级总览

| 阶段 | 改动项 | 收益 | 风险 | 建议时机 |
|------|--------|------|------|----------|
| P1 | `taskexecutor` → `scheduler` | 中 | 极低 | 立即 |
| P1 | 去掉 `_service` 文件名后缀 | 低 | 极低 | 立即 |
| P1 | 合并 `pkg/apperr` → `core/errors` | 中 | 低 | 立即 |
| P2 | 拆分 `shein/operation/` | 高 | 中 | ✅ 已完成 |
| P2 | 新建 `shein/context/`，修复 `GetTask()` | 高 | 中 | 下个迭代 |
| P2 | 拆分 `amazon/core/service/` | 中 | 中 | ✅ 已完成 |
| P3 | `domain/model/` → `model/` | 高 | 高 | 专项 Sprint |
| P3 | 消灭 `app/di/` | 中 | 高 | 专项 Sprint |
| P3 | 整理 `app/messaging/` | 中 | 高 | 专项 Sprint |
