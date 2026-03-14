# 包结构重构计划

> 基于 Go 语言编码规范，针对 `internal/` 目录的包组织问题制定的重构计划。

---

## 问题汇总

| # | 问题 | 严重程度 |
|---|------|----------|
| 1 | `internal/app` 与 `internal/application` 并存，职责重叠 | 高 |
| 2 | `internal/pkg/utils` 是语义不明的杂货铺包 | 高 |
| 3 | `internal/infra/repo` 与 `internal/infra/repository` 并存 | 中 |
| 4 | `internal/domain/product` 内部按技术层分包（factory/repo/service/types） | 中 |
| 5 | `internal/platforms/temu` 内部按技术层分包（api/services/handlers/utils） | 中 |
| 6 | `internal/core` 与 `internal/pkg` 边界模糊，无明确分层标准 | 低 |

---

## 目标结构

```
internal/
├── core/           # 基础设施：config、logger、errors、metrics、lifecycle（保持不变）
├── domain/         # 领域模型与业务规则（按业务聚合，不按技术层）
├── crawler/        # 爬虫实现（保持不变）
├── platforms/      # 平台对接（内部按业务聚合重组）
├── infra/          # 基础设施实现（合并 repo/repository，清理重复）
├── pipeline/       # 流水线（保持不变）
└── pkg/            # 内部公共工具库（清理 utils，按职责拆分）
```

> 合并 `internal/app` + `internal/application` → 统一为 `internal/app`，删除 `internal/application`。

---

## 分阶段执行计划

### 阶段一：合并 `app` 与 `application`（优先级：高）

**目标**：消除两个职责重叠的目录，统一为 `internal/app`。

**映射关系**：

| 原路径 | 目标路径 | 说明 |
|--------|----------|------|
| `internal/application/crawler_amazon/` | `internal/app/crawler/amazon/` | 合并到 app 下的 crawler 子目录 |
| `internal/application/crawler1688/` | `internal/app/crawler/alibaba1688/` | 同上 |
| `internal/application/distributed_crawler/` | `internal/app/crawler/distributed/` | 同上 |
| `internal/application/product_fetcher/` | `internal/app/task/` | 合并到已有的 task 包 |
| `internal/application/productjson/` | `internal/app/productjson/` | 与已有 productjson 合并 |
| `internal/application/state/` | `internal/app/state/` | 直接迁移 |

**步骤**：
1. 逐个迁移子包，更新 import 路径
2. 运行 `go build ./...` 验证编译
3. 删除 `internal/application/` 目录

---

### 阶段二：拆解 `internal/pkg/utils`（优先级：高）

**目标**：将 `utils` 中的 20 个文件按职责归入具名包。

**拆分映射**：

| 原文件 | 目标包 | 新路径 |
|--------|--------|--------|
| `http_client.go` | `httpclient` | `internal/pkg/httpclient/` |
| `image_utils.go` | `imageutil` | `internal/pkg/imageutil/` |
| `goroutine_manager.go` | `goroutine` | `internal/pkg/goroutine/` |
| `parallel_processor.go` | `goroutine` | `internal/pkg/goroutine/` |
| `shutdown.go` | `goroutine` | `internal/pkg/goroutine/` |
| `sku_generator.go` | `skugen` | `internal/pkg/skugen/` |
| `text_cleaner.go` | `strutil` | 合并到已有 `internal/pkg/strutil/` |
| `hash_utils.go` | `hashutil` | `internal/pkg/hashutil/` |
| `json_utils.go` | `jsonutil` | 合并到已有 `internal/pkg/jsonutil/` |
| `cache.go` | `cacheutil` | `internal/pkg/cacheutil/` |
| `errors.go` | `core/errors` | 合并到 `internal/core/errors/` |
| `logger.go` | `core/logger` | 合并到 `internal/core/logger/` |
| `file_utils.go` | `fileutil` | `internal/pkg/fileutil/` |
| `platform_utils.go` | `platform` 所属包 | 按具体内容归入对应平台包 |
| `instance_utils.go` | 按内容判断 | 迁移后评估 |
| `performance_tracker.go` | `metrics` | 合并到 `internal/core/metrics/` |
| `task_metrics.go` | `metrics` | 合并到 `internal/core/metrics/` |
| `version_utils.go` | `version` | `internal/pkg/version/` |

**步骤**：
1. 每次迁移一个文件，立即更新所有 import
2. 每迁移完一批，运行 `go build ./...` 验证
3. 所有文件迁移完成后删除 `internal/pkg/utils/`

---

### 阶段三：合并 `infra/repo` 与 `infra/repository`（优先级：中）

**目标**：消除重复目录，统一使用 `internal/infra/repository`。

**步骤**：
1. 将 `internal/infra/repo/file_repo.go` 移入 `internal/infra/repository/`
2. 更新 import 路径
3. 删除 `internal/infra/repo/` 目录

---

### 阶段四：重组 `domain/product` 内部结构（优先级：中）

**目标**：消除按技术层分包（factory/repo/service/types），改为在包内按文件职责组织。

**当前结构**：
```
internal/domain/product/
├── factory/
├── repo/
├── service/
├── types/
├── cache_manager.go
├── data_parser.go
└── ...
```

**目标结构**：
```
internal/domain/product/
├── product.go          # 核心类型与接口定义
├── factory.go          # 工厂函数（原 factory/ 内容）
├── repo.go             # 仓储接口（原 repo/ 内容）
├── service.go          # 领域服务（原 service/ 内容）
├── cache_manager.go
├── data_parser.go
└── ...
```

**步骤**：
1. 将各子目录的文件提升到父包，调整 package 声明
2. 解决可能的命名冲突
3. 删除空子目录

---

### 阶段五：重组 `platforms/temu` 内部结构（优先级：中）

**目标**：与阶段四同理，消除 api/services/handlers/utils 的技术层分包。

**当前结构**：
```
internal/platforms/temu/
├── api/
├── services/
├── handlers/
├── utils/
├── context/
├── types/
├── task_executor/
└── *.go
```

**目标结构**：
```
internal/platforms/temu/
├── temu.go             # 入口与核心类型
├── api.go              # API 调用（原 api/ 内容）
├── handler.go          # 消息处理（原 handlers/ 内容）
├── service.go          # 业务逻辑（原 services/ 内容）
├── executor.go         # 任务执行（原 task_executor/ 内容）
└── ...
```

对 `internal/platforms/shein` 做同样处理。

---

### 阶段六：明确 `core` 与 `pkg` 的边界（优先级：低）

**约定**：

- `internal/core/`：框架级基础设施，所有业务包都依赖它（config、logger、errors、metrics、lifecycle）。不包含任何业务逻辑。
- `internal/pkg/`：业务工具库，提供可复用的业务辅助能力（watermark、pricing、resilience 等）。可以依赖 `core`，但不能依赖 `domain` 或 `app`。

在 `internal/core/README.md` 和 `internal/pkg/README.md` 中补充上述约定说明。

---

## 执行原则

1. **每次只改一个包**，改完立即编译验证，不要批量操作
2. **先改 import，再删旧文件**，避免编译中断
3. **不改业务逻辑**，本次重构只移动文件和调整包名
4. **每个阶段完成后**，运行 `go vet ./...` 和现有测试确认无回归

---

## 预期收益

- 消除歧义目录，新代码有明确的归属位置
- `utils` 拆解后，依赖关系更清晰，便于单独测试
- 平台包内部结构统一，降低跨平台开发的认知切换成本
- 符合团队 Go 编码规范，减少 code review 争议
