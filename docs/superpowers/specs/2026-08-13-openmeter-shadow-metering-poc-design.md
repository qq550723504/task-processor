# OpenMeter 影子计量 PoC 设计

## 1. 状态与决策

- 日期：2026-08-13
- 校准基线：`a029af1ab14a628b091e3bc884db088b6ae7479f`
- 状态：设计方向已确认，等待书面规范复核后进入实施计划
- 核心决策：先完成一个不接入生产、不改变收费和 entitlement 行为的 OpenMeter 契约 PoC；只有 PoC 通过后，才在付费试点既定 PAY-041 顺序中设计本地 usage event/outbox、影子投递和后续切换。

本设计只回答一件事：OpenMeter 是否适合作为 ListingKit 后续通用计量与 entitlement 引擎。它不提前实施 PAY-041～PAY-044，不改变当前商业放行顺序，也不启用收费。

## 2. 背景

当前 `internal/listingsubscription` 已经拥有模块、套餐、租户订阅、entitlement、月度 usage counter 和审计记录。现有 `CheckUsage` 先调用 `AuthorizeUsage`，再调用 `RecordUsage`；授权读取和计数写入不是一个原子操作。部分 ListingKit 调用点还忽略 `RecordUsage` 错误。这套实现适合早期功能门禁，但继续扩展会逐步重新实现幂等事件、聚合、额度、对账和账单基础设施。

付费试点产品目录已经冻结三类对 PoC 有代表性的计量语义：

1. `studio_design_jobs_succeeded`：成功完成后增加一次的离散计数。
2. `shein_drafts_succeeded`：远端草稿确认成功后增加一次的离散计数。
3. `storage_bytes_current`：租户当前保留对象的存量值，上传增加、删除降低，不是累计上传量。

付费试点执行计划把 PAY-041～PAY-044 放在身份/租户隔离、SHEIN 提交安全和 1688 商业闭环之后。因此当前可以做的工作只能是隔离的技术验证，不能把生产授权或计量读取切到 OpenMeter。

OpenMeter 提供官方 Go SDK、CloudEvents 事件摄取、COUNT/LATEST 等 meter、entitlement 和用量查询。PoC 固定使用与被测 OpenMeter Open Source release 匹配的官方 Go SDK，不手写 OpenMeter API DTO、REST 客户端或第二套协议模型，也不使用仅面向托管产品的 API surface。

## 3. 目标

### 3.1 功能目标

- 验证三个代表性指标能否被 OpenMeter 准确表达和查询。
- 验证稳定业务事件重复投递不会重复计量。
- 验证 tenant 到 OpenMeter subject 的映射不会造成跨租户聚合。
- 验证 `storage_bytes_current` 在上传、删除和乱序事件下的实际语义。
- 验证 OpenMeter 不可用后，调用方使用同一事件身份重放能否恢复正确结果。
- 验证 metered entitlement 的余额和访问结果能否满足展示与普通门禁需要。
- 用并发实验确认 OpenMeter access check 是否能独立承担硬配额原子预留；不能满足时，明确本地 reservation 的最小责任。

### 3.2 决策目标

PoC 结束时必须形成以下三种结论之一，不能只记录“API 调通”：

1. **采用为计量和 entitlement 引擎**：计量、隔离、重放、存量值和 entitlement 均满足要求；硬配额是否仍需本地 reservation 单独记录。
2. **仅采用为计量引擎**：事件与聚合满足要求，但 entitlement 或硬门禁行为不满足；本地继续拥有 entitlement/reservation。
3. **拒绝采用**：幂等、租户隔离、存量值或故障重放中的任一核心语义无法可靠满足，且没有窄而稳定的适配方案。

## 4. 非目标

- 不修改任何生产 API、handler、Temporal Workflow 或 RabbitMQ consumer。
- 不向生产、staging 或现有共享 Kubernetes 集群部署 OpenMeter。
- 不双写现有 `saas_usage_counters`。
- 不创建 PAY-041 的生产 usage ledger 或 outbox 表。
- 不把现有套餐、tenant subscription 或人工商业台账迁移到 OpenMeter。
- 不启用 Stripe、支付、自动发票或客户收费。
- 不删除 `CheckUsage`、`AuthorizeUsage`、`RecordUsage` 或现有 counter。
- 不全仓替换 GORM `AutoMigrate`，也不在本 PoC 中引入生产 goose migration。
- 不比较 Flexprice；除非 OpenMeter 被拒绝，才单独启动 Flexprice PoC。
- 不做 OpenMeter 生产容量规划、HA 或灾备设计；只记录其自托管依赖和后续生产评估项。

## 5. 方案比较与选择

### 5.1 方案 A：直接把生产计量切到 OpenMeter

优点是最快得到真实流量，缺点是会把当前缺少事务 outbox、远端未知状态处理和静默写入失败的问题原样带入新系统，同时违反付费试点 PR 顺序。本方案拒绝。

### 5.2 方案 B：先全仓迁移数据库治理，再开始计量 PoC

优点是 schema 管理更规范，缺点是范围会扩展到大量无关 `AutoMigrate` 和历史表，形成新的基础设施重构项目，无法回答 OpenMeter 是否适配。本方案暂缓；未来只在 PAY-041 中先用 goose 管理新 usage/outbox 表，再逐步迁移存量表。

### 5.3 方案 C：隔离契约 PoC，之后通过 outbox 影子接入

优点是不会改变现有行为，能够先验证最不确定的外部系统语义，并为后续 PAY-041 提供明确边界。缺点是 PoC 结果不代表生产可用性，后续仍需部署和运维评估。本方案被选中。

## 6. PoC 边界与组件

PoC 由四个隔离组件组成：

1. **上游 OpenMeter quickstart 环境**：使用官方仓库提供的 Docker Compose quickstart，在开发机或一次性 CI 环境运行。仓库不复制或维护 Kafka、ClickHouse、PostgreSQL 的自定义 PoC 编排。
2. **OpenMeter 集成适配器**：位于 `internal/integration/openmeter`，使用官方 Go SDK；仅暴露 PoC 需要的事件摄取、meter 查询和 entitlement 查询能力，不进入应用 bootstrap。
3. **契约测试**：位于同一集成目录，只在 `OPENMETER_POC=1` 时运行；此时 `OPENMETER_POC_URL` 必填，`OPENMETER_API_KEY` 仅在被测环境要求认证时注入。默认 `go test ./...` 不访问 OpenMeter，并把契约测试明确报告为 skip。
4. **结果报告**：写入 `docs/architecture/openmeter-shadow-metering-poc-report.md`，记录上游版本、镜像 digest、命令、完整结果、失败证据、资源占用和最终采用结论。

PoC 不新增常驻命令、不注册 HTTP route、不增加生产配置字段。后续若采用 OpenMeter，生产适配器是否继续位于该目录由实施计划决定，但领域接口必须由订阅/计量领域拥有，不能让业务 handler 直接依赖 OpenMeter SDK。

## 7. 事件合同

### 7.1 CloudEvent 公共字段

所有 PoC 事件使用以下合同：

```json
{
  "specversion": "1.0",
  "type": "listingkit.usage",
  "source": "task-processor/listingkit",
  "id": "<stable-business-event-id>",
  "subject": "tenant:<tenant-id>",
  "time": "<business-occurred-at-rfc3339>",
  "data": {
    "metric": "<catalog-metric>",
    "quantity": "<decimal-string>",
    "source_type": "<domain-object-type>",
    "source_id": "<domain-object-id>",
    "revision": "<domain-event-revision>"
  }
}
```

规则：

- `subject` 只使用稳定 tenant ID，不使用用户名、邮箱、店铺名或可变显示名。
- `id` 由业务事实确定，重试和恢复必须复用；不得在每次 HTTP 调用时重新生成随机 ID。
- `source + id` 在所有租户和指标中唯一。推荐逻辑形式为 `tenant_id/metric/source_type/source_id/revision`，具体编码必须可逆或有独立审计字段。
- `time` 是业务事实发生时间，不是异步投递时间。
- 数量使用十进制字符串，避免跨语言数值精度差异。
- `data` 不包含商品标题、图片、Prompt、合同价格、个人信息或平台凭据。

### 7.2 Meter 映射

| 产品指标 | OpenMeter meter | 聚合 | 值来源 | 时间窗口 |
| --- | --- | --- | --- | --- |
| `studio_design_jobs_succeeded` | 同名 | `COUNT` | 匹配 `data.metric` 的事件 | 月度查询 |
| `shein_drafts_succeeded` | 同名 | `COUNT` | 匹配 `data.metric` 的事件 | 月度查询 |
| `storage_bytes_current` | 同名 | `LATEST` | `data.quantity` | 查询时最新业务时间 |

两个成功指标只发送 committed success 事实。失败、取消、平台拒绝和工程重试不创建新的客户用量事件。

`storage_bytes_current` 每次发送租户最新总保留字节数，而不是单次上传或删除 delta。这样 LATEST meter 的值直接表达当前占用，避免漏掉一次负 delta 后永久漂移。生产实现如何事务性计算和投递快照不在本 PoC 范围内，但 PoC 必须验证乱序事件不会让旧快照覆盖新快照；若 OpenMeter 按摄取顺序而不是业务时间选择 LATEST，则该指标不得直接迁移。

## 8. Entitlement 实验

PoC 创建三个 feature：

- Studio 成功任务：月度 metered entitlement，额度 5。
- SHEIN 草稿成功：月度 metered entitlement，额度 3。
- 当前存储：metered entitlement，额度 10 MiB。

实验分别验证：

- 零用量、部分用量、达到额度和超过额度时的 usage、balance、overage、hasAccess。
- 两个租户使用同一 feature 时额度和用量完全隔离。
- 月度窗口查询与产品目录语义一致。
- 存储量下降后 entitlement 余额恢复。

硬配额并发实验使用“剩余额度为 1，20 个并发调用先查询 access、再提交不同业务事件”的场景。预期该实验可能证明普通 access check 不能提供与本地业务事务绑定的原子预留。此结果不自动拒绝 OpenMeter作为计量引擎，但必须把本地 reservation/commit/release 记录为后续 PAY-041 的强制边界，禁止用客户端锁或进程内 mutex 掩盖问题。

## 9. 故障与重放实验

至少覆盖以下场景：

1. 同一 `source + id` 顺序重放 10 次，聚合只增加一次。
2. 同一 ID 使用不同 source，记录为不同事件，用于验证身份边界，而不是作为生产重试方式。
3. 事件提交收到超时或连接中断后，使用完全相同事件重试，结果不重复。
4. OpenMeter 停止期间保留一组确定性事件；恢复后重放，最终聚合等于预期。
5. 两个 tenant 使用相同 source object ID，仍因完整事件 ID 和 subject 不同而隔离。
6. 新存储快照先到、旧快照后到，最终值仍必须是业务时间较新的快照。
7. 非法 metric、缺少 tenant、负存储值和非十进制 quantity 被适配器或 OpenMeter明确拒绝，不得静默接受。

PoC 不实现生产 outbox，但报告必须把每个故障场景映射到未来 outbox dispatcher 的重试、死信和人工补记责任。

## 10. 测试结构

### 10.1 单元测试

- 稳定事件 ID 构造和输入规范化。
- tenant subject 构造。
- 产品 metric 到 OpenMeter meter 的映射。
- 禁止敏感字段进入事件 data。
- OpenMeter 错误分类：可重试、永久拒绝、认证/配置失败。

单元测试不需要 OpenMeter，必须进入默认 Go 测试集。

### 10.2 集成契约测试

- 真实 OpenMeter quickstart 环境。
- 真实官方 Go SDK。
- 每个测试使用唯一前缀，避免 meter/customer/subject 相互污染。
- 测试结束只清理本次命名空间内可安全删除的资源；不执行广泛数据库或 Docker volume 删除。
- 如果 `OPENMETER_POC` 未设置为 `1`，测试必须报告 skip；如果已启用但 URL、初始化资源或依赖服务缺失，测试必须失败，不能退回 skip 或伪造通过。

### 10.3 报告证据

报告至少包含：

- OpenMeter release/tag、Git SHA 和镜像 digest。
- Docker Compose 渲染结果及实际启动的服务清单。
- 每项测试的命令、通过/失败数量和关键响应摘要。
- Kafka、ClickHouse、PostgreSQL、OpenMeter 进程的空闲和测试峰值资源占用。
- 所有语义偏差、规避方案及其是否需要本地状态。
- 最终三选一采用结论。

## 11. 通过门禁

以下条件全部满足，PoC 才能建议至少采用 OpenMeter 作为计量引擎：

```text
[ ] 三个 meter 的正常场景聚合与期望完全一致。
[ ] 相同 source + id 重放不会重复计量。
[ ] tenant subject 查询不存在跨租户数据。
[ ] 服务中断后的确定性重放不会少记或多记。
[ ] 非法事件不会被静默计量。
[ ] storage LATEST 按业务时间保持最新快照；否则明确判定该指标不适配。
[ ] entitlement 普通查询满足余额和访问展示语义。
[ ] 并发硬配额实验有可复现结论，并明确是否需要本地 reservation。
[ ] 自托管依赖和资源开销被完整记录，没有把 quickstart 当成生产部署。
[ ] 未修改任何生产运行时行为，默认测试不会依赖外部 OpenMeter。
```

任何幂等、租户隔离或故障重放失败均为拒绝采用的硬失败。硬配额原子预留失败不是计量引擎硬失败，但会把本地 reservation 设为后续设计的强制要求。存储 LATEST 失败只允许把该指标留在本地，不能用忽略乱序的方式通过。

## 12. PoC 通过后的生产演进边界

PoC 通过不授权生产接入。后续必须按照付费试点顺序另行设计和批准：

1. PAY-041 使用 goose 为新 `usage_events`、`usage_outbox` 和必要的 reservation 表建立增量迁移；不全仓替换 AutoMigrate。
2. 业务成功和 outbox 写入处于同一数据库事务。
3. dispatcher 使用稳定事件身份投递 OpenMeter，记录 attempt、last error 和 delivered timestamp。
4. 现有 counter 与 OpenMeter 先影子对账，不驱动收费。
5. PAY-042 才统一所有入口的 entitlement 和 usage 行为。
6. PAY-044 建立每日差异、补记和人工调整审计。
7. 连续对账窗口、故障演练和商业门禁通过后，才单独决定读取切换和旧 counter 删除。

本地始终拥有业务事件何时成功、远端结果是否确定、reservation/commit/release、合同订单关联和人工调整审计。OpenMeter 拥有通用事件聚合、meter、额度计算和后续账单能力。两者不能各自维护一套不同的业务成功定义。

## 13. 安全与数据边界

- PoC 使用隔离的开发凭据和测试 tenant，不使用生产客户数据。
- OpenMeter endpoint、API key 和任何连接串仅通过环境注入，不写入仓库。
- 日志和报告不得包含 API key、数据库密码、用户邮箱、商品内容或平台凭据。
- 自托管 quickstart 只绑定开发机需要的端口；不得暴露到公网。
- 报告只保留事件 ID、虚构 tenant、metric、数量和错误类别。
- OpenMeter 不获得 ZITADEL token、Casbin policy、店铺凭据或 marketplace access token。

## 14. 可观测性

PoC 记录以下最小指标或结构化统计：

- event ingest 请求数、成功数、可重试失败、永久失败；
- meter query 请求数、结果和延迟；
- entitlement query 请求数、结果和延迟；
- 重放事件数与最终去重结果；
- 每个 tenant/metric 的期望值和实际值；
- 外部组件资源占用。

这些统计只用于决策报告，不在 PoC 中新增 Prometheus dashboard 或生产告警。

## 15. 实施计划输入

用户复核本设计后，实施计划必须把工作拆成可独立验证的步骤：

1. 建立不接 bootstrap 的 OpenMeter 官方 Go SDK 适配器和单元测试。
2. 建立三类 meter/feature/entitlement 的可重复初始化 fixture。
3. 实现正常、幂等、租户隔离、故障重放、LATEST 乱序和并发额度契约测试。
4. 使用官方 quickstart 运行完整验证并记录不可变上游版本。
5. 生成 PoC 报告，给出采用、仅计量采用或拒绝采用结论。

任何生产 handler 修改、部署 manifest、outbox schema、双写或切流都必须从实施计划中排除，并在 PAY-041 到达既定门禁后作为新的设计处理。
